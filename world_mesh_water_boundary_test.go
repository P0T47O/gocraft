package main

import "testing"

func TestMissingMeshNeighborKeepsProceduralWater(t *testing.T) {
	const seed uint32 = 123452
	// Find a submerged point on a chunk's east edge and its missing halo.
	var wx, wz int
	found := false
	for cx := -32; cx <= 32 && !found; cx++ {
		for cz := -32; cz <= 32 && !found; cz++ {
			for z := 0; z < chunkWidth; z++ {
				x, candidateZ := cx*chunkWidth+chunkWidth, cz*chunkWidth+z
				if sampleTerrainColumn(seed, x, candidateZ).blockAt(seed, x, seaLevel-1, candidateZ) == blockWater {
					wx, wz, found = x, candidateZ, true
					break
				}
			}
		}
	}
	if !found {
		t.Fatal("no submerged east boundary found")
	}
	job := meshJob{seed: seed, baseX: wx - chunkWidth, baseZ: divFloor(wz, chunkWidth) * chunkWidth,
		centerCX: divFloor(wx, chunkWidth) - 1, centerCZ: divFloor(wz, chunkWidth),
		yMin: seaLevel - sectionHeight, yMax: seaLevel}
	snapshot := buildMeshSnapshotFromNeighbors(job)
	if got := snapshot.blockAt(wx, seaLevel-1, wz); got != blockWater {
		t.Errorf("unloaded ocean neighbor: got %d, want water", got)
	}
	snapshot.Release()

	// A received chunk is authoritative even when a player has drained it.
	w := NewClientWorld()
	neighbor := w.ensureChunk(job.centerCX+1, job.centerCZ)
	job.neighbors[2][1] = neighbor
	snapshot = buildMeshSnapshotFromNeighbors(job)
	if got := snapshot.blockAt(wx, seaLevel-1, wz); got != blockAir {
		t.Errorf("received empty neighbor: got %d, want air", got)
	}
	snapshot.Release()
	w.Close()
}
