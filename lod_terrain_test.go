package main

import (
	"strconv"
	"testing"
)

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
				height, _, color := lodGridColumn(1234511, wx, wz, x == 0 || x == lodCellsPerTile || z == 0 || z == lodCellsPerTile, nil)
				if v.Position[0] != float32(wx)-.5 || v.Position[2] != float32(wz)-.5 || v.Position[1] != height-.5 || v.Color != color {
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

func TestLODHandoffSamplesEveryVoxelOnRealChunkEdge(t *testing.T) {
	const seed uint32 = 1234511
	columns := make(map[lodPoint]lodColumn)
	for _, p := range []lodPoint{{-1, 7}, {16, 7}} {
		c := lodSampleColumn(seed, p.X, p.Z, nil)
		c.observed = true
		columns[p] = c
	}
	tile := buildLODTileAtStep(seed, lodTileKey{0, 0}, columns, lodNearCellSize)
	for _, edge := range []struct {
		marker float32
		x, z   int
		innerX int
		shift  float32
	}{
		{-1, 0, 7, -1, -.35},
		{-3, 16, 7, 16, .35},
	} {
		want := float32(lodSampleColumn(seed, edge.innerX, edge.z, nil).height) - .52
		found := false
		for _, vertex := range tile.vertices {
			if vertex.Texcoord[1] == edge.marker && vertex.Position[0] == float32(edge.x)-.5+edge.shift && vertex.Position[2] == float32(edge.z)-.5 {
				if vertex.Position[1] != want {
					t.Fatalf("edge %v at z=%d: height %.2f, want %.2f", edge.marker, edge.z, vertex.Position[1], want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("edge %v missed odd voxel z=%d", edge.marker, edge.z)
		}
	}
}

func TestLODHandoffUsesClimateGrassAndSeparateWater(t *testing.T) {
	const seed = uint32(1234511)
	climate := climateColor(seed, 128, 256)
	grass := lodSurfaceColorAt(seed, 128, 256, blockGrass)
	if grass != [4]uint8{uint8(float32(climate.R) * (147.0 / 255.0)), uint8(float32(climate.G) * (147.0 / 255.0)), uint8(float32(climate.B) * (147.0 / 255.0)), 255} {
		t.Fatalf("LOD grass lost full-detail climate tint: %v", grass)
	}
	foundWater := false
	for z := -2; z <= 2 && !foundWater; z++ {
		for x := -2; x <= 2 && !foundWater; x++ {
			tile := buildLODTileAtStep(seed, lodTileKey{x, z}, nil, lodFarCellSize)
			if len(tile.waterIndices) == 0 {
				continue
			}
			foundWater = true
			for _, index := range tile.waterIndices {
				if int(index) >= len(tile.waterVertices) {
					t.Fatalf("water index %d outside %d vertices", index, len(tile.waterVertices))
				}
			}
			for _, vertex := range tile.waterVertices {
				if vertex.Color[3] != 200 {
					t.Fatalf("LOD water must match full-detail surface alpha: %v", vertex.Color)
				}
			}
		}
	}
	if !foundWater {
		t.Fatal("fixture did not exercise a water tile")
	}
}

func TestLODBoundaryStripJoinsRealColumnWithoutChangingUnmaskedGrid(t *testing.T) {
	const seed = uint32(1234511)
	columns := make(map[lodPoint]lodColumn)
	for _, p := range []lodPoint{{15, 0}, {0, 15}, {16, 0}, {0, 16}} {
		c := lodSampleColumn(seed, p.X, p.Z, nil)
		c.observed = true
		columns[p] = c
	}
	tile := buildLODTileAtStep(seed, lodTileKey{0, 0}, columns, lodNearCellSize)
	wantX := float32(lodSampleColumn(seed, 15, 0, nil).height) - .52
	wantZ := float32(lodSampleColumn(seed, 0, 15, nil).height) - .52
	wantEast := float32(lodSampleColumn(seed, 16, 0, nil).height) - .52
	wantSouth := float32(lodSampleColumn(seed, 0, 16, nil).height) - .52
	foundX, foundZ, foundEast, foundSouth := false, false, false, false
	for _, v := range tile.vertices {
		if v.Texcoord[1] == -1 && v.Position[0] == 15.5-.35 && v.Position[2] == -.5 {
			foundX = true
			if v.Position[1] != wantX {
				t.Fatalf("X handoff misses last real column: got %v, want %v", v.Position[1], wantX)
			}
		}
		if v.Texcoord[1] == -2 && v.Position[0] == -.5 && v.Position[2] == 15.5-.35 {
			foundZ = true
			if v.Position[1] != wantZ {
				t.Fatalf("Z handoff misses last real column: got %v, want %v", v.Position[1], wantZ)
			}
		}
		if v.Texcoord[1] == -3 && v.Position[0] == 15.5+.35 && v.Position[2] == -.5 {
			foundEast = true
			if v.Position[1] != wantEast {
				t.Fatalf("east handoff misses first real column: got %v, want %v", v.Position[1], wantEast)
			}
		}
		if v.Texcoord[1] == -4 && v.Position[0] == -.5 && v.Position[2] == 15.5+.35 {
			foundSouth = true
			if v.Position[1] != wantSouth {
				t.Fatalf("south handoff misses first real column: got %v, want %v", v.Position[1], wantSouth)
			}
		}
	}
	if !foundX || !foundZ || !foundEast || !foundSouth {
		t.Fatalf("missing chunk-edge transition: X=%t Z=%t east=%t south=%t", foundX, foundZ, foundEast, foundSouth)
	}
}

func TestLODMixedLevelEdgesStayOnTheSameLine(t *testing.T) {
	const seed = uint32(1234511)
	for _, left := range []lodTileKey{{0, 0}, {-5, 3}} {
		fine := buildLODTileAtStep(seed, left, nil, lodNearCellSize)
		coarse := buildLODTileAtStep(seed, lodTileKey{left.X + 1, left.Z}, nil, lodFarCellSize)
		fineSide := lodTileSize/lodNearCellSize + 1
		coarseSide := lodTileSize/lodFarCellSize + 1
		for z := 0; z <= lodTileSize; z += lodNearCellSize {
			v := fine.vertices[(z/lodNearCellSize)*fineSide+fineSide-1]
			lo := coarse.vertices[(z/lodFarCellSize)*coarseSide]
			hi := coarse.vertices[min(z/lodFarCellSize+1, coarseSide-1)*coarseSide]
			fraction := float32(z%lodFarCellSize) / lodFarCellSize
			wantY := lerp(lo.Position[1], hi.Position[1], fraction)
			if abs(v.Position[1]-wantY) > .0001 || v.Position[0] != lo.Position[0] || v.Position[2] != float32(left.Z*lodTileSize+z)-.5 {
				t.Fatalf("mixed 4/16 grid seam at %v z=%d: got=%v expected height=%v", left, z, v.Position, wantY)
			}
		}
		for _, tile := range []lodTileData{fine, coarse} {
			for _, index := range tile.indices {
				if int(index) >= len(tile.vertices) {
					t.Fatalf("mixed-level index %d outside %d vertices", index, len(tile.vertices))
				}
			}
		}
	}
}

func TestLODParentMorphMatchesNextLevel(t *testing.T) {
	const seed = uint32(1234511)
	for _, step := range []int{lodNearCellSize, lodTransitionCellSize, lodCellSize} {
		for _, key := range []lodTileKey{{0, 0}, {-5, 3}} {
			fine := buildLODTileAtStep(seed, key, nil, step)
			coarse := buildLODTileAtStep(seed, key, nil, step*2)
			fineSide := lodTileSize/step + 1
			coarseSide := lodTileSize/(step*2) + 1
			for z := 0; z < fineSide; z++ {
				for x := 0; x < fineSide; x++ {
					got := fine.vertices[z*fineSide+x]
					if got.Texcoord[1] != float32(step) {
						t.Fatalf("step %d tile %v (%d,%d): missing morph level", step, key, x, z)
					}
					cx, cz := x/2, z/2
					v := func(px, pz int) float32 { return coarse.vertices[pz*coarseSide+px].Position[1] }
					want := v(cx, cz)
					if x%2 != 0 && z%2 != 0 {
						want = (v(cx+1, cz) + v(cx, cz+1)) * .5
					} else if x%2 != 0 {
						want = (want + v(cx+1, cz)) * .5
					} else if z%2 != 0 {
						want = (want + v(cx, cz+1)) * .5
					}
					if abs(got.Texcoord[0]-want) > .0001 {
						t.Fatalf("step %d tile %v (%d,%d): target=%v parent=%v", step, key, x, z, got.Texcoord[0], want)
					}
				}
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

func BenchmarkLODTileLevels(b *testing.B) {
	for _, step := range []int{lodNearCellSize, lodCellSize, lodFarCellSize} {
		b.Run(strconv.Itoa(step), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				buildLODTileAtStep(1234511, lodTileKey{i % 16, i / 16}, nil, step)
			}
		})
	}
}
