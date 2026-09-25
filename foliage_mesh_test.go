package main

import "testing"

func TestLeafCanopyCullsSharedFaces(t *testing.T) {
	initBlockRegistry()
	a := &RenderAssets{}
	for _, leaf := range []byte{blockLeaves, blockLeavesBirch, blockLeavesSpruce} {
		for _, neighbor := range []byte{blockLeaves, blockLeavesBirch, blockLeavesSpruce} {
			for _, pair := range [][2]litFace{{lightTop, lightBottom}, {lightSouth, lightNorth}, {lightEast, lightWest}} {
				if !a.shouldDrawVoxelFace(leaf, neighbor, pair[0]) || a.shouldDrawVoxelFace(leaf, neighbor, pair[1]) {
					t.Fatalf("shared leaf face %d -> %d was not drawn exactly once (%v)", leaf, neighbor, pair)
				}
			}
		}
		if !a.shouldDrawFace(leaf, blockAir) || !a.shouldDrawFace(leaf, blockGlass) {
			t.Fatalf("exposed leaf face %d was hidden", leaf)
		}
	}
	if !a.shouldDrawFace(blockGlass, blockGlass) {
		t.Fatal("glass behavior changed with leaf culling")
	}

	var heights [chunkWidth][chunkWidth]int16
	data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
		func(x, y, z int) byte {
			if y == 8 && z == 8 {
				switch x {
				case 8:
					return blockLeaves
				case 9:
					return blockLeavesBirch
				}
			}
			return blockAir
		},
		func(int, int, int) byte { return 15 },
		func(int, int, int) byte { return 0 }, 42)
	defer releaseMeshResults(data)
	faces := 0
	for _, list := range data["cutout"] {
		for _, mesh := range list {
			faces += mesh.vertCount / 4
		}
	}
	if faces != 11 { // Two cubes with one double-sided shared face.
		t.Fatalf("adjacent leaves emitted %d faces, want 11", faces)
	}
}
