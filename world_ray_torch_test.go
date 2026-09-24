package main

import "testing"

func TestTorchRayUsesRenderedShape(t *testing.T) {
	initBlockRegistry()
	for meta := byte(0); meta <= 4; meta++ {
		t.Run(string(rune('0'+meta)), func(t *testing.T) {
			w := NewClientWorld()
			defer w.Close()
			chunk := lifecycleChunk(w, chunkKey{0, 0})
			chunk.blocks.Set(8, 8, 8, blockTorch)
			chunk.meta.Set(8, 8, 8, meta)
			chunk.blocks.Set(8, 8, 9, blockStone)

			g := geometryForTorch(meta)
			front := meshVec3(8+g.offset.X, 8+g.offset.Y, 6)
			forward := meshVec3(0, 0, 1)
			if hit := w.HitTest(front, forward, 4); !hit.hit || hit.x != 8 || hit.y != 8 || hit.z != 8 || hit.distance <= 0 {
				t.Fatalf("meta %d: center ray should hit torch: %+v", meta, hit)
			}

			// The same voxel contains empty space beside the slender model.
			// A mining ray through it must reach the stone beyond.
			miss := w.HitTest(meshVec3(8.44, 8.44, 6), forward, 4)
			if !miss.hit || miss.x != 8 || miss.y != 8 || miss.z != 9 {
				t.Fatalf("meta %d: ray beside torch should hit rear stone: %+v", meta, miss)
			}
			miss = w.HitTest(meshVec3(8.44, 8.44, 8), forward, 2)
			if !miss.hit || miss.z != 9 {
				t.Fatalf("meta %d: ray starting in torch voxel should reach rear stone: %+v", meta, miss)
			}
			if hit := w.HitTest(front, forward, 1); hit.hit {
				t.Fatalf("meta %d: torch beyond reach was selected: %+v", meta, hit)
			}
		})
	}
}
