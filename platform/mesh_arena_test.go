package platform

import (
	"math/rand"
	"testing"
)

func TestMeshArenaRangesReuseWithoutOverlap(t *testing.T) {
	const capacity = 1024
	free := []meshRange{{0, capacity}}
	var live []meshRange
	rng := rand.New(rand.NewSource(91))
	for step := 0; step < 4000; step++ {
		if len(live) > 0 && rng.Intn(2) == 0 {
			i := rng.Intn(len(live))
			free = freeMeshRange(free, live[i].offset, live[i].count)
			live = append(live[:i], live[i+1:]...)
		} else {
			count := uint32(rng.Intn(70) + 1)
			if i := findMeshRange(free, count); i >= 0 {
				live = append(live, meshRange{takeMeshRange(&free, i, count), count})
			}
		}
		var occupied [capacity]bool
		for _, ranges := range [][]meshRange{live, free} {
			for _, r := range ranges {
				if r.count == 0 || r.offset+r.count > capacity {
					t.Fatal("invalid range", r)
				}
				for i := r.offset; i < r.offset+r.count; i++ {
					if occupied[i] {
						t.Fatal("overlapping range", step, r)
					}
					occupied[i] = true
				}
			}
		}
		for _, used := range occupied {
			if !used {
				t.Fatal("lost capacity", step)
			}
		}
	}
	for _, r := range live {
		free = freeMeshRange(free, r.offset, r.count)
	}
	if len(free) != 1 || free[0] != (meshRange{0, capacity}) {
		t.Fatal("failed to coalesce", free)
	}
}

func TestMeshArenaRetainedUntilLastMesh(t *testing.T) {
	before := MeshBufferBytes.Load()
	b := &WebGPUMeshBackend{}
	p := &meshArenaPage{owner: b, refs: 2, bytes: 100}
	b.pages = []*meshArenaPage{p}
	MeshBufferBytes.Add(100)
	p.release(0, 2, 0, 3)
	if len(b.pages) != 1 || MeshBufferBytes.Load() != before+100 {
		t.Fatal("released a live shared buffer")
	}
	p.release(2, 2, 3, 3)
	if len(b.pages) != 0 || MeshBufferBytes.Load() != before {
		t.Fatal("last reference leaked")
	}
}
