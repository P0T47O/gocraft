package main

import "testing"

type unloadTestMesh struct{ calls int }

func (*unloadTestMesh) IndexCount() int32 { return 3 }
func (m *unloadTestMesh) Unload()         { m.calls++ }
func TestChunkMeshUnloadOnce(t *testing.T) {
	handle := &unloadTestMesh{}
	mesh := &ChunkMesh{gpuMesh: handle}
	mesh.unload()
	mesh.unload()
	if handle.calls != 1 || mesh.gpuMesh != nil {
		t.Fatal("mesh handle release is not idempotent")
	}
}
