package main

import "gocraft/platform"

// ChunkMesh retains only an opaque GPU handle and legacy binding IDs.
// No Raylib material pointers enter shared world/mesh ownership.
type ChunkMesh struct {
	glMesh              platform.MeshHandle
	textureID, shaderID uint32
}

func (m *ChunkMesh) unload() {
	if m.glMesh != nil {
		m.glMesh.Unload()
		m.glMesh = nil
	}
}
