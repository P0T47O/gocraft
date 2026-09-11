package main

import (
	"slices"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestRenderRadiusOffsetsCachedAndUnique(t *testing.T) {
	var cache worldRenderCache
	offsets := cache.radiusOffsets(2)
	if len(offsets) != 13 {
		t.Fatalf("radius 2: got %d offsets, want 13", len(offsets))
	}
	seen := make(map[[2]int]bool)
	for i, item := range offsets {
		key := [2]int{item.dx, item.dz}
		if seen[key] {
			t.Fatalf("duplicate offset: %v", key)
		}
		seen[key] = true
		if item.dx*item.dx+item.dz*item.dz > 4 {
			t.Fatalf("outside radius: %v", key)
		}
		if i > 0 && offsets[i-1].dist > item.dist {
			t.Fatal("offsets are not near-to-far")
		}
	}
	if &offsets[0] != &cache.radiusOffsets(2)[0] {
		t.Fatal("unchanged radius reallocates")
	}
	if allocs := testing.AllocsPerRun(100, func() { cache.radiusOffsets(2) }); allocs != 0 {
		t.Fatalf("cached offsets allocate: %v", allocs)
	}
	if got := len(cache.radiusOffsets(0)); got != 1 {
		t.Fatalf("radius change: got %d, want 1", got)
	}
}

func TestVisibleSectionsCullEmptyAndOutside(t *testing.T) {
	var cache worldRenderCache
	chunk := new(Chunk)
	chunk.sectionBlocks[0] = 1
	chunk.sectionBlocks[1] = 1
	// The plane accepts y <= 10, excluding section 1. Other zero planes
	// impose no restriction, making this independent of a graphics context.
	frustum := Frustum{}
	frustum.Planes[0] = mgl32.Vec4{0, -1, 0, 10}
	cache.appendVisibleSections(chunk, 0, 0, &frustum, mgl32.Vec3{})
	if len(cache.visible) != 1 || cache.visible[0].sec != 0 {
		t.Fatalf("expected only occupied section 0, got %+v", cache.visible)
	}
	cache.visible = cache.visible[:0]
	// A narrow view intersects the negative half-block edge at x=-0.5.
	frustum.Planes[0] = mgl32.Vec4{-1, 0, 0, -0.25}
	cache.appendVisibleSections(chunk, 0, 0, &frustum, mgl32.Vec3{})
	if len(cache.visible) != 2 {
		t.Fatal("block-centered bounds culled visible edges")
	}
}

func TestTranslucentSectionsInterleaveWaterGlass(t *testing.T) {
	chunk := new(Chunk)
	ensureChunkSections(chunk)
	chunk.waterMeshes[0] = map[string][]*ChunkMesh{"water": {nil}}
	chunk.glassMeshes[0] = map[string][]*ChunkMesh{"glass": {nil}}
	chunk.glassMeshes[1] = map[string][]*ChunkMesh{"glass": {nil}}
	chunk.waterMeshes[2] = map[string][]*ChunkMesh{}
	cache := worldRenderCache{visible: []visibleSection{
		{chunk: chunk, sec: 0, dist: 1},
		{chunk: chunk, sec: 1, dist: 9},
		{chunk: chunk, sec: 2, dist: 16},
	}}
	cache.collectTranslucent()
	if len(cache.translucent) != 3 {
		t.Fatalf("got %d draws; empty maps must be skipped", len(cache.translucent))
	}
	if first := cache.translucent[0]; first.section.sec != 1 || !first.glass {
		t.Fatal("far glass must precede near water")
	}
	if cache.translucent[1].glass || !cache.translucent[2].glass {
		t.Fatal("same-section water/glass tie is unstable")
	}
	if allocs := testing.AllocsPerRun(100, func() { cache.collectTranslucent() }); allocs != 0 {
		t.Fatalf("reused translucent list allocates: %v", allocs)
	}
}

func TestRenderSectionOrderingUsesHeightAndStableTies(t *testing.T) {
	a := visibleSection{cx: 1, cz: 0, sec: 1, dist: 4}
	b := visibleSection{cx: 0, cz: 0, sec: 2, dist: 4}
	c := visibleSection{cx: 0, cz: 0, sec: 0, dist: 1}
	sections := []visibleSection{a, b, c}
	slices.SortFunc(sections, compareVisibleSections)
	if sections[0] != c || sections[1] != b || sections[2] != a {
		t.Fatal("unexpected near/tie ordering")
	}
	chunk := new(Chunk)
	chunk.sectionBlocks[0], chunk.sectionBlocks[2] = 1, 1
	var cache worldRenderCache
	cache.appendVisibleSections(chunk, 0, 0, &Frustum{}, mgl32.Vec3{7.5, 40, 7.5})
	slices.SortFunc(cache.visible, compareVisibleSections)
	if cache.visible[0].sec != 2 {
		t.Fatal("section distance ignores camera height")
	}
}
