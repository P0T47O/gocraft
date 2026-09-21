package main

import "gocraft/platform"

// ChunkMesh retains only a backend-owned GPU handle.
type ChunkMesh struct {
	gpuMesh platform.MeshHandle
}

func (m *ChunkMesh) unload() {
	if m.gpuMesh != nil {
		m.gpuMesh.Unload()
		m.gpuMesh = nil
	}
}
