package main

import "testing"

func TestLODTileMatchesTerrainColumnsAndNeighbors(t *testing.T) {
	for _, key := range []lodTileKey{{0, 0}, {-3, 2}, {7, -9}} {
		got := buildLODTile(1234511, key)
		const side = lodCellsPerTile + 1
		if len(got.vertices) < side*side || len(got.indices) < lodCellsPerTile*lodCellsPerTile*6 {
			t.Fatalf("%v: incomplete surface mesh", key)
		}
		for z := 0; z < side; z++ {
			for x := 0; x < side; x++ {
				v := got.vertices[z*side+x]
				wx, wz := key.X*lodTileSize+x*lodCellSize, key.Z*lodTileSize+z*lodCellSize
				column := sampleTerrainColumn(1234511, wx, wz)
				if v.Position[0] != float32(wx) || v.Position[2] != float32(wz) || v.Position[1] != float32(column.height)-.65 || v.Color != lodSurfaceColor(column.top) {
					t.Fatalf("%v vertex (%d,%d) does not match generator: %+v", key, x, z, v)
				}
			}
		}
		neighbor := buildLODTile(1234511, lodTileKey{key.X + 1, key.Z})
		for z := 0; z < side; z++ {
			if got.vertices[z*side+lodCellsPerTile] != neighbor.vertices[z*side] {
				t.Fatalf("%v east seam differs at row %d", key, z)
			}
		}
		for _, index := range got.indices {
			if int(index) >= len(got.vertices) {
				t.Fatalf("%v: index %d outside %d vertices", key, index, len(got.vertices))
			}
		}
	}
}

func TestLODTileDeterministicAndNoWorldMutation(t *testing.T) {
	key := lodTileKey{-5, -4}
	a, b := buildLODTile(42, key), buildLODTile(42, key)
	if len(a.vertices) != len(b.vertices) || len(a.indices) != len(b.indices) {
		t.Fatal("same seed changed mesh dimensions")
	}
	for i := range a.vertices {
		if a.vertices[i] != b.vertices[i] {
			t.Fatalf("vertex %d changed", i)
		}
	}
	for i := range a.indices {
		if a.indices[i] != b.indices[i] {
			t.Fatalf("index %d changed", i)
		}
	}
}

func TestLODTreeSilhouettesAreBoundedAndTagged(t *testing.T) {
	count := 0
	for z := -2; z <= 2; z++ {
		for x := -2; x <= 2; x++ {
			key := lodTileKey{x, z}
			tile := buildLODTile(1234511, key)
			for _, v := range tile.vertices {
				if v.Color[3] != 254 {
					continue
				}
				count++
				if v.Position[0] < float32(x*lodTileSize)-3 || v.Position[0] > float32((x+1)*lodTileSize)+3 ||
					v.Position[2] < float32(z*lodTileSize)-3 || v.Position[2] > float32((z+1)*lodTileSize)+3 {
					t.Fatalf("tree vertex outside tile margin: key=%v vertex=%v", key, v.Position)
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("sample region contains no distant tree silhouettes")
	}
}

func TestLODTreePositionsMatchFullGenerator(t *testing.T) {
	const seed = uint32(1234511)
	key := lodTileKey{0, 0}
	tile := buildLODTile(seed, key)
	actual := make(map[[3]int]bool)
	treeStart := -1
	for i, v := range tile.vertices {
		if v.Color[3] != 254 {
			continue
		}
		if treeStart < 0 {
			treeStart = i
		}
		if (i-treeStart)%17 == 0 {
			// Each tree starts with the trunk's bottom south-west corner.
			actual[[3]int{int(v.Position[0] + .35), int(v.Position[1] + .5), int(v.Position[2] + .35)}] = true
		}
	}
	expected := make(map[[3]int]bool)
	for z := key.Z * lodTileSize; z < (key.Z+1)*lodTileSize; z++ {
		for x := key.X * lodTileSize; x < (key.X+1)*lodTileSize; x++ {
			if tree, ok := sampleTreeAnchor(seed, x, z); ok {
				expected[[3]int{tree.x, tree.y, tree.z}] = true
			}
		}
	}
	if len(expected) == 0 || len(actual) != len(expected) {
		t.Fatalf("LOD tree count=%d, full generator=%d", len(actual), len(expected))
	}
	for pos := range expected {
		if !actual[pos] {
			t.Fatalf("missing exact full-generator tree at %v", pos)
		}
	}
}

func TestHorizonDistanceClamp(t *testing.T) {
	for _, tc := range [][2]int{{-1, 0}, {0, 0}, {1, 32}, {64, 64}, {200, 128}} {
		if got := clampHorizonDistance(tc[0]); got != tc[1] {
			t.Fatalf("%d: got %d want %d", tc[0], got, tc[1])
		}
	}
}

func BenchmarkLODTileBuild(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildLODTile(1234511, lodTileKey{i % 16, i / 16})
	}
}
