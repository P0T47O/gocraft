package main

import (
	"math"
	"testing"
)

func TestFacingAndShapeMetadata(t *testing.T) {
	initBlockRegistry()
	for _, tc := range []struct {
		yaw  float32
		face byte
	}{{0, faceNorth}, {math.Pi / 2, faceWest}, {math.Pi, faceSouth}, {-math.Pi / 2, faceEast}} {
		if got := facingFromYaw(tc.yaw); got != tc.face {
			t.Fatalf("yaw %.2f faces %d, wanted %d", tc.yaw, got, tc.face)
		}
		if got := placementMeta(blockOakStairs, tc.yaw, true); got != tc.face|shapeUpper {
			t.Fatalf("upside stair metadata %d", got)
		}
	}
	if placementMeta(blockOakSlab, 0, true) != shapeUpper || validPlacementMeta(blockOakSlab, 3) || validPlacementMeta(blockChest, 7) {
		t.Fatal("partial block metadata validation failed")
	}
	for face := byte(0); face < 4; face++ {
		textures := orientedTextures(blockFurnace, face, GetBlock(blockFurnace).Textures)
		front := "textures/block/furnace_front.png"
		seen := 0
		for _, path := range [...]string{textures.North, textures.East, textures.South, textures.West} {
			if path == front {
				seen++
			}
		}
		if seen != 1 {
			t.Fatalf("furnace face %d has %d fronts", face, seen)
		}
	}
}

func TestShapedBlockRayAndCollision(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks.Set(8, 8, 8, blockOakSlab)
	c.blocks.Set(8, 8, 9, blockStone)
	forward := meshVec3(0, 0, 1)
	if hit := w.HitTest(meshVec3(8, 8.3, 6), forward, 5); !hit.hit || hit.z != 9 {
		t.Fatalf("ray did not pass above lower slab: %+v", hit)
	}
	if hit := w.HitTest(meshVec3(8, 7.8, 6), forward, 5); !hit.hit || hit.z != 8 {
		t.Fatalf("ray missed lower slab: %+v", hit)
	}
	if colliderHitsCore(w, meshVec3(8, 8.05, 8), Collider{Width: .5, Depth: .5, Height: 1}) {
		t.Fatal("lower slab blocked empty upper half")
	}
	if !colliderHitsCore(w, meshVec3(8, 7.8, 8), Collider{Width: .5, Depth: .5, Height: 1}) {
		t.Fatal("lower slab collider missing")
	}
	c.meta.Set(8, 8, 8, shapeUpper)
	if hit := w.HitTest(meshVec3(8, 7.8, 6), forward, 5); !hit.hit || hit.z != 9 {
		t.Fatalf("ray did not pass below upper slab: %+v", hit)
	}
	if hit := w.HitTest(meshVec3(8, 8.3, 6), forward, 5); !hit.hit || hit.z != 8 {
		t.Fatalf("ray missed upper slab: %+v", hit)
	}
	for face := byte(0); face < 4; face++ {
		boxes, n := shapeBoxes(blockOakStairs, face)
		if n != 2 || boxes[0].maxY != 0 {
			t.Fatalf("stairs %d base shape invalid: %+v", face, boxes)
		}
	}
}

func TestSlabAutoStep(t *testing.T) {
	w := lifeTestWorld(t)
	w.SetBlockAt(8, 71, 9, blockOakSlab)
	p := newGameVec3(8, 72.125, 7.8)
	var state InputState
	for i := 0; i < 20; i++ {
		p = state.stepMovementCore(w, p, 1.0/60, MovementControls{Forward: 1}, false)
	}
	if p.Z < 8.8 || p.Y < 72.5 {
		t.Fatalf("player could not step onto slab: %+v", p)
	}
}

func TestAutoStepDoesNotClimbFullBlock(t *testing.T) {
	w := lifeTestWorld(t)
	w.SetBlockAt(8, 71, 9, blockStone)
	p := newGameVec3(8, 72.125, 7.8)
	var state InputState
	for i := 0; i < 20; i++ {
		p = state.stepMovementCore(w, p, 1.0/60, MovementControls{Forward: 1}, false)
	}
	if p.Z > 8.25 || p.Y > 72.3 {
		t.Fatalf("auto-step climbed a full block: %+v", p)
	}
}

func TestStairAutoStepFromFront(t *testing.T) {
	w := lifeTestWorld(t)
	w.SetBlockAt(8, 71, 9, blockOakStairs)
	w.SetMetaAt(8, 71, 9, faceNorth)
	p := newGameVec3(8, 72.125, 7.8)
	var state InputState
	for i := 0; i < 20; i++ {
		p = state.stepMovementCore(w, p, 1.0/60, MovementControls{Forward: 1}, false)
	}
	if p.Z < 8.8 || p.Y < 73.0 {
		t.Fatalf("player could not climb stair: %+v", p)
	}
}

func TestShapedMeshRespectsHalfHeight(t *testing.T) {
	initBlockRegistry()
	a := &RenderAssets{}
	var heights [chunkWidth][chunkWidth]int16
	for _, meta := range []byte{0, shapeUpper} {
		data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
			func(x, y, z int) byte {
				if x == 8 && y == 8 && z == 8 {
					return blockStoneSlab
				}
				return blockAir
			},
			func(int, int, int) byte { return 15 },
			func(int, int, int) byte { return meta }, 1)
		meshes := data["opaque"]["textures/block/stone.png"]
		if len(meshes) != 1 || meshes[0].vertCount != 24 {
			t.Fatalf("slab %d missing six faces", meta)
		}
		low, high := float32(100), float32(-100)
		for i := 1; i < len(meshes[0].vertices); i += 3 {
			low = min(low, meshes[0].vertices[i])
			high = max(high, meshes[0].vertices[i])
		}
		if meta == 0 && (low != 7.5 || high != 8) || meta != 0 && (low != 8 || high != 8.5) {
			t.Fatalf("slab %d vertices span %.2f..%.2f", meta, low, high)
		}
		releaseMeshResults(data)
	}
}

func TestStairMeshesHaveOutwardFaces(t *testing.T) {
	initBlockRegistry()
	a := &RenderAssets{}
	var heights [chunkWidth][chunkWidth]int16
	for meta := byte(0); meta < 8; meta++ {
		data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
			func(x, y, z int) byte {
				if x == 8 && y == 8 && z == 8 {
					return blockOakStairs
				}
				return blockAir
			},
			func(int, int, int) byte { return 15 },
			func(int, int, int) byte { return meta }, 1)
		meshes := data["opaque"]["textures/block/oak_planks.png"]
		if len(meshes) != 1 || meshes[0].vertCount != 44 {
			t.Fatalf("stairs meta %d has wrong face count", meta)
		}
		mesh := meshes[0]
		for f := 0; f < mesh.vertCount/4; f++ {
			i := f * 12
			a := meshVec3(mesh.vertices[i+3]-mesh.vertices[i], mesh.vertices[i+4]-mesh.vertices[i+1], mesh.vertices[i+5]-mesh.vertices[i+2])
			b := meshVec3(mesh.vertices[i+6]-mesh.vertices[i], mesh.vertices[i+7]-mesh.vertices[i+1], mesh.vertices[i+8]-mesh.vertices[i+2])
			n := meshCross(a, b)
			if n.X*mesh.normals[i]+n.Y*mesh.normals[i+1]+n.Z*mesh.normals[i+2] <= 0 {
				t.Fatalf("stairs meta %d face %d has inward winding", meta, f)
			}
		}
		releaseMeshResults(data)
	}
}

func TestServerOwnsPlacedFacing(t *testing.T) {
	s, p := hostileTestServer(t)
	p.GameMode = ModeCreative
	p.Yaw = math.Pi / 2
	for _, tc := range []struct {
		id       byte
		x        int32
		meta     byte
		wantMeta byte
	}{
		{blockFurnace, 8, faceNorth, faceWest},
		{blockChest, 9, faceSouth, faceWest},
		{blockOakStairs, 10, shapeUpper, faceWest | shapeUpper},
		{blockOakSlab, 11, shapeUpper, shapeUpper},
	} {
		s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: tc.x, Y: 71, Z: 8, BlockID: tc.id, Meta: tc.meta}})
		if id, meta := s.World.BlockAt(int(tc.x), 71, 8), s.World.MetaAt(int(tc.x), 71, 8); id != tc.id || meta != tc.wantMeta {
			t.Fatalf("placed %d with id/meta %d/%d, wanted %d/%d", tc.id, id, meta, tc.id, tc.wantMeta)
		}
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 12, Y: 71, Z: 8, BlockID: blockOakSlab, Meta: 0xff}})
	if s.World.BlockAt(12, 71, 8) != blockAir {
		t.Fatal("server accepted invalid slab metadata")
	}
}

func TestSlabMergeConsumesOneItemAndDropsTwo(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Inventory.Slots[0] = Item{ID: int32(blockOakSlab), Count: 1}
	s.World.SetBlockAt(8, 71, 8, blockOakSlab)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockOakSlab, Meta: shapeDouble}})
	if s.World.MetaAt(8, 71, 8) != shapeDouble || p.Inventory.Slots[0].Count != 0 || slabDropCount(blockOakSlab, shapeDouble) != 2 {
		t.Fatal("slab merge did not consume one slab and preserve two drops")
	}
	boxes, count := shapeBoxes(blockOakSlab, shapeDouble)
	if count != 1 || boxes[0].minY != -.5 || boxes[0].maxY != .5 {
		t.Fatal("double slab is not a full collision box")
	}
	s.World.SetMetaAt(8, 71, 8, shapeUpper)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockOakSlab, Meta: shapeDouble}})
	if s.World.MetaAt(8, 71, 8) != shapeUpper {
		t.Fatal("slab merged with no item")
	}
}

func TestStairCornersFollowNeighborAndKeepPassage(t *testing.T) {
	for _, test := range []struct {
		name   string
		nx, nz int
		count  int
	}{
		{"outer", 0, 1, 2},
		{"inner", 0, -1, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			block := func(x, y, z int) byte {
				if y == 8 && x == test.nx && z == test.nz {
					return blockOakStairs
				}
				return blockAir
			}
			meta := func(x, y, z int) byte { return faceEast }
			boxes, count := shapeBoxesAt(blockOakStairs, faceNorth, 0, 8, 0, block, meta)
			if count != test.count {
				t.Fatalf("boxes = %d, want %d", count, test.count)
			}
			for _, box := range boxes[1:count] {
				if box.minX >= box.maxX || box.minY >= box.maxY || box.minZ >= box.maxZ {
					t.Fatalf("degenerate riser: %+v", box)
				}
			}
		})
	}
}

func TestStairCornerUsesNeighborMetadataAcrossChunks(t *testing.T) {
	w := lifeTestWorld(t)
	left := lifecycleChunk(w, chunkKey{0, 0})
	right := lifecycleChunk(w, chunkKey{1, 0})
	w.SetBlockAt(15, 71, 8, blockOakStairs)
	w.SetMetaAt(15, 71, 8, faceEast)
	w.SetBlockAt(16, 71, 8, blockOakStairs)
	w.SetMetaAt(16, 71, 8, faceNorth)
	job := meshJob{baseX: 0, baseZ: 0, centerCX: 0, centerCZ: 0, yMin: 64, yMax: 80}
	job.neighbors[1][1], job.neighbors[2][1] = left, right
	snapshot := buildMeshSnapshotFromNeighbors(job)
	defer snapshot.Release()
	_, count := shapeBoxesAt(blockOakStairs, faceEast, 15, 71, 8, snapshot.blockAt, snapshot.metaAt)
	if count != 3 {
		t.Fatalf("cross-chunk stair is not an inner corner: %d boxes", count)
	}
}

func TestDoubleSlabBlocksAndRestoresSkyLight(t *testing.T) {
	w := lifeTestWorld(t)
	c := w.getChunkIfGenerated(0, 0)
	for y := 71; y <= 75; y++ {
		for x := 7; x <= 9; x++ {
			for z := 7; z <= 9; z++ {
				if x != 8 || z != 8 {
					c.blocks.Set(x, y, z, blockStone)
				}
			}
		}
	}
	initializeChunkLighting(c)
	w.SetBlockAt(8, 74, 8, blockOakSlab)
	if got := w.LightSkyAt(8, 72, 8); got != 15 {
		t.Fatalf("single slab closed skylight: %d", got)
	}
	w.SetMetaAt(8, 74, 8, shapeDouble)
	if got := w.LightSkyAt(8, 72, 8); got != 0 {
		t.Fatalf("double slab leaked skylight: %d", got)
	}
	w.SetMetaAt(8, 74, 8, 0)
	if got := w.LightSkyAt(8, 72, 8); got != 15 {
		t.Fatalf("splitting double slab did not restore skylight: %d", got)
	}
}
