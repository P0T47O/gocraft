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
	return distance * 0.70, distance - min(1.5*float32(chunkWidth), distance*0.15)
}

// World owns this cache through its render worldRenderCache field. All scratch
// slices retain capacity, but release chunk references at the end of each frame.
type worldRenderCache struct {
	drawCalls, triangles int
	offsets              []chunkItem
	radius               int
	visible              []visibleSection
	translucent          []translucentDraw
	paths                []string
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
	for sec := 0; sec < sectionCount; sec++ {
		if chunk.sectionBlocks[sec] == 0 {
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
