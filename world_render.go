package main

import (
	"cmp"
	"slices"

	"github.com/go-gl/mathgl/mgl32"
)

const worldRenderRadius = defaultRenderDistance

// Cover the requested horizontal radius plus vertical relief and grid margin.
// Keep the old near/default-distance projection range unchanged.
func worldFarPlane(radius int) float32 {
	return float32(max(1000, radius*chunkWidth+chunkHeight+2*chunkWidth))
}

// Leave most of the view clear, with a soft fade before the circular chunk
// boundary. The margin covers chunk-grid rounding and camera motion in a chunk.
func worldFogRange(radius int) (start, end float32) {
	distance := float32(max(radius, 2) * chunkWidth)
	end = distance - min(1.5*float32(chunkWidth), distance*0.15)
	// Clear weather: reserve only the outer fringe for hiding missing terrain.
	// This follows Minecraft's separation of boundary and environment fog,
	// not a byte-for-byte copy of any particular version's constants.
	return end - min(distance*0.12, 4*float32(chunkWidth)), end
}

// World owns this cache through its render worldRenderCache field. All scratch
// slices retain capacity, but release chunk references at the end of each frame.
type worldRenderCache struct {
	drawCalls, triangles           int
	visibleSections, readySections int
	offsets                        []chunkItem
	radius                         int
	visible                        []visibleSection
	translucent                    []translucentDraw
	paths                          []string
}

type visibleSection struct {
	chunk       *Chunk
	cx, cz, sec int
	dist        float32
}

type translucentDraw struct {
	section visibleSection
	glass   bool
}

// Distant underground interiors cost mesh work and draw calls even though
// terrain completely hides them from an exterior camera. Keep one full
// section of safety below the lowest column; nearby caves remain intact.
func distantBuriedSection(lowest int16, sec int, horizontalDistanceSq float32) bool {
	const caveDetailRadius = 24 * chunkWidth
	if horizontalDistanceSq <= caveDetailRadius*caveDetailRadius {
		return false
	}
	return (sec+1)*sectionHeight < int(lowest)-sectionHeight
}

func (r *worldRenderCache) radiusOffsets(radius int) []chunkItem {
	if r.offsets != nil && r.radius == radius {
		return r.offsets
	}
	r.radius = radius
	r.offsets = r.offsets[:0]
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			dist := dx*dx + dz*dz
			if dist <= radius*radius {
				r.offsets = append(r.offsets, chunkItem{dx: dx, dz: dz, dist: float64(dist)})
			}
		}
	}
	slices.SortFunc(r.offsets, func(a, b chunkItem) int {
		if order := cmp.Compare(a.dist, b.dist); order != 0 {
			return order
		}
		if order := cmp.Compare(a.dz, b.dz); order != 0 {
			return order
		}
		return cmp.Compare(a.dx, b.dx)
	})
	return r.offsets
}

// Explicit coordinate ties give deterministic ordering even when distance ties
// change with camera movement; they do not depend on map iteration order.
func compareVisibleSections(a, b visibleSection) int {
	if order := cmp.Compare(a.dist, b.dist); order != 0 {
		return order
	}
	return compareSectionCoordinates(a, b)
}

func compareSectionCoordinates(a, b visibleSection) int {
	if order := cmp.Compare(a.cz, b.cz); order != 0 {
		return order
	}
	if order := cmp.Compare(a.cx, b.cx); order != 0 {
		return order
	}
	return cmp.Compare(a.sec, b.sec)
}

func compareTranslucentDraws(a, b translucentDraw) int {
	if order := cmp.Compare(b.section.dist, a.section.dist); order != 0 {
		return order
	}
	if order := compareSectionCoordinates(a.section, b.section); order != 0 {
		return order
	}
	if a.glass == b.glass {
		return 0
	}
	if a.glass {
		return 1
	}
	return -1 // Water precedes glass only within the same section/distance.
}

func (r *worldRenderCache) appendVisibleSections(chunk *Chunk, chunkX, chunkZ int, frustum *Frustum, camPos mgl32.Vec3) {
	centerX := float32(chunkX*chunkWidth) + (chunkWidth-1)*0.5
	centerZ := float32(chunkZ*chunkWidth) + (chunkWidth-1)*0.5
	dx, dz := camPos.X()-centerX, camPos.Z()-centerZ
	horizontalDistanceSq := dx*dx + dz*dz
	lowest := int16(chunkHeight)
	if horizontalDistanceSq > (24*chunkWidth)*(24*chunkWidth) {
		for x := range chunk.heightMap {
			for _, h := range chunk.heightMap[x] {
				if h < lowest {
					lowest = h
				}
			}
		}
	}
	for sec := 0; sec < sectionCount; sec++ {
		if chunk.sectionBlocks[sec] == 0 {
			continue
		}
		if distantBuriedSection(lowest, sec, horizontalDistanceSq) {
			continue
		}
		// Blocks are centered at integer positions, so bounds extend half a block
		// beyond the first center (including torches and translucent geometry).
		min := mgl32.Vec3{float32(chunkX*chunkWidth) - 0.5, float32(sec*sectionHeight) - 0.5, float32(chunkZ*chunkWidth) - 0.5}
		max := min.Add(mgl32.Vec3{chunkWidth, sectionHeight, chunkWidth})
		if !frustum.IntersectsAABB(min, max) {
			continue
		}
		center := min.Add(max).Mul(0.5)
		delta := camPos.Sub(center)
		r.visible = append(r.visible, visibleSection{chunk: chunk, cx: chunkX, cz: chunkZ, sec: sec, dist: delta.Dot(delta)})
	}
}

func (r *worldRenderCache) collectTranslucent() {
	clear(r.translucent)
	r.translucent = r.translucent[:0]
	for _, section := range r.visible {
		if len(section.chunk.waterMeshes[section.sec]) != 0 {
			r.translucent = append(r.translucent, translucentDraw{section: section})
		}
		if len(section.chunk.glassMeshes[section.sec]) != 0 {
			r.translucent = append(r.translucent, translucentDraw{section: section, glass: true})
		}
	}
	slices.SortFunc(r.translucent, compareTranslucentDraws)
}
