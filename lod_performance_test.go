package main

import "testing"

func TestLODUnknownTileHasNoRealChunkSeams(t *testing.T) {
	mesh := buildLODTileAtStep(1234511, lodTileKey{}, nil, lodNearCellSize)
	for _, v := range mesh.vertices {
		if v.Texcoord[1] < 0 {
			t.Fatal("unknown tile contains unused real-chunk seams")
		}
	}
}

func lodDenseFoliageFixture() map[lodPoint]lodColumn {
	columns := make(map[lodPoint]lodColumn)
	for z := 0; z < 32; z++ {
		for x := 0; x < 32; x++ {
			columns[lodPoint{x, z}] = lodColumn{height: 60, observed: true, occupancy: &lodOccupancy{ground: 60, spans: []lodSpan{{80, 84, blockLeaves}}}}
		}
	}
	return columns
}

func TestLODObservedFoliageGeometryBudget(t *testing.T) {
	columns := lodDenseFoliageFixture()
	var data lodTileData
	appendLODOccupancy(&data, lodTileKey{}, columns)
	// 64 crown cells, 12 triangles each. Previously this fixture emitted 4352
	// triangles (per-column caps and exterior walls).
	if len(data.indices)/3 != 768 {
		t.Fatalf("crown budget exceeded: %d triangles", len(data.indices)/3)
	}
	for _, c := range columns {
		c.occupancy.spans[0].block = blockGoldBlock
	}
	var reference lodTileData
	appendLODOccupancy(&reference, lodTileKey{}, columns)
	if len(reference.indices)/3 != 4352 {
		t.Fatalf("voxel reference changed: %d triangles", len(reference.indices)/3)
	}
	for _, v := range data.vertices {
		if v.Position[1] < 79.5 || v.Position[1] > 83.5 {
			t.Fatal("crown extended into an air gap")
		}
	}
	clear(columns)
	data = lodTileData{}
	appendLODOccupancy(&data, lodTileKey{}, columns)
	if len(data.vertices) != 0 {
		t.Fatal("removed foliage returned")
	}
}

func TestLODObservedPatchSelection(t *testing.T) {
	for _, step := range []int{2, 4, 8, 16} {
		for _, p := range []lodPoint{{-1, 0}, {0, 0}, {7, 9}, {127, 128}, {129, 128}} {
			patches := lodObservedPatchCells(lodTileKey{}, step, map[lodPoint]lodColumn{p: {observed: true, changed: true}})
			cells := lodTileSize / step
			for z := 0; z < cells; z++ {
				for x := 0; x < cells; x++ {
					want := p.X >= x*step-1 && p.X <= x*step+step+1 && p.Z >= z*step-1 && p.Z <= z*step+step+1
					if patches[z*cells+x] != want {
						t.Fatalf("step %d point %v cell %d,%d", step, p, x, z)
					}
				}
			}
		}
	}
}

func BenchmarkLODObservedFoliage(b *testing.B) {
	for _, name := range []string{"Crown", "VoxelReference"} {
		b.Run(name, func(b *testing.B) {
			columns := lodDenseFoliageFixture()
			if name == "VoxelReference" {
				for _, c := range columns {
					c.occupancy.spans[0].block = blockGoldBlock
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				var data lodTileData
				appendLODOccupancy(&data, lodTileKey{}, columns)
			}
		})
	}
}
