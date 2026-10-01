package main

import (
	"testing"
	"time"
)

func TestMeshPrewarmReservesQueueAndBuildsBothDirections(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	for _, key := range []chunkKey{{0, -1}, {0, 1}} {
		c := lifecycleChunk(w, key)
		c.blocks.Set(8, 32, 8, blockStone)
		c.sectionBlocks[2] = 1
		c.tints = &meshTintCache{}
		for x := range c.heightMap {
			for z := range c.heightMap[x] {
				c.heightMap[x][z] = 33
			}
		}
		c.invalidateMeshSection(2)
	}
	for i := 0; i < 8; i++ {
		w.meshJobs <- meshJob{snapshot: &meshSnapshot{}}
	}
	w.prewarmMeshes(0, 0, 2, &RenderAssets{}, time.Now().Add(time.Second))
	if len(w.meshJobs) != 8 {
		t.Fatal("background work consumed reserved queue space")
	}
	for len(w.meshJobs) > 0 {
		<-w.meshJobs
	}
	w.prewarmMeshes(0, 0, 2, &RenderAssets{}, time.Now().Add(time.Second))
	if len(w.meshJobs) != 2 {
		t.Fatal("did not warm front and back surfaces", len(w.meshJobs))
	}
	for len(w.meshJobs) > 0 {
		job := <-w.meshJobs
		if job.section != 2 || job.urgent || job.snapshot == nil {
			t.Fatal("invalid prewarm snapshot")
		}
		job.snapshot.Release()
	}
}
