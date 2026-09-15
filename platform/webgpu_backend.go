//go:build webgpu

package platform

import "fmt"

// WebGPUMeshBackend is the experimental replacement for the OpenGL mesh path.
// The build tag keeps the incomplete backend out of normal builds while the
// device/surface implementation is brought up.
type WebGPUMeshBackend struct{}

func (WebGPUMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	return &pendingWebGPUMesh{vertexCount: len(vertices), indexCount: len(indices)}
}

type pendingWebGPUMesh struct {
	vertexCount int
	indexCount  int
}

func (m *pendingWebGPUMesh) Draw() {
	panic(fmt.Sprintf("WebGPU backend not initialized: attempted to draw mesh with %d vertices / %d indices", m.vertexCount, m.indexCount))
}

func (m *pendingWebGPUMesh) Unload() {}

// EnableExperimentalWebGPU switches future mesh uploads to the experimental
// backend. It is deliberately explicit while surface/device bring-up is still
// incomplete; normal builds continue to use OpenGL.
func EnableExperimentalWebGPU() {
	SetMeshBackend(WebGPUMeshBackend{})
}
