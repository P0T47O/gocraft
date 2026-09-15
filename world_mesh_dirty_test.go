package main

import "testing"

func TestMeshChangeHaloExact(t *testing.T) {
	for _, tc := range []struct{ x, y, z, count int }{{8, 20, 8, 1}, {15, 20, 8, 2}, {15, 31, 15, 8}, {0, 0, 0, 4}, {15, 255, 15, 4}} {
		var changes chunkMeshChanges
		changes.mark(tc.x, tc.y, tc.z)
		count := 0
		for dx := 0; dx < 3; dx++ {
			for dz := 0; dz < 3; dz++ {
				for sec := 0; sec < sectionCount; sec++ {
					if changes[dx][dz]&(1<<sec) != 0 {
						count++
					}
				}
			}
		}
		if count != tc.count {
			t.Fatalf("%+v: got %d", tc, count)
		}
	}
}

func TestMeshUpdatesCoalesceButRejectRunningSnapshot(t *testing.T) {
	c := new(Chunk)
	ensureChunkSections(c)
	c.invalidateMeshSection(0)
	version := c.meshVersion[0]
	for i := 0; i < 100; i++ {
		c.invalidateMeshSection(0)
	}
	if c.meshVersion[0] != version {
		t.Fatal("unscheduled changes were not merged")
	}
	c.pendingOpaque[0] = true
	c.meshSubmittedVersion[0] = version
	for i := 0; i < 100; i++ {
		c.invalidateMeshSection(0)
	}
	if c.meshVersion[0] != version+1 {
		t.Fatal("running job must be invalidated once")
	}
	c.meshSubmittedVersion[0] = c.meshVersion[0]
	c.invalidateMeshSection(0)
	if c.meshVersion[0] != version+2 {
		t.Fatal("new snapshot not invalidated")
	}
}

func TestInteriorChunkRefreshLeavesNeighborsAlone(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			c := lifecycleChunk(w, chunkKey{dx, dz})
			clear(c.sectionDirty)
		}
	}
	c := w.getChunkIfGenerated(0, 0)
	p := chunkPacket(chunkKey{}, c)
	p.Data[(8*chunkHeight+20)*chunkWidth+8] = blockStone
	if !w.applyChunkPacket(p) {
		t.Fatal("packet rejected")
	}
	for key, c := range w.chunks {
		for sec, v := range c.sectionDirty {
			want := key == (chunkKey{}) && sec == 1
			if v != want {
				t.Fatalf("dirty %v sec %d=%v want %v", key, sec, v, want)
			}
		}
	}
}
