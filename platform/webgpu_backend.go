//go:build webgpu

package platform

// WebGPUMeshBackend is the experimental replacement for the OpenGL mesh path.
// Device/surface and real GPU buffers are intentionally the next milestone.
type WebGPUMeshBackend struct{}

func (WebGPUMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	// Keep an owned CPU copy during bring-up. This makes the backend seam usable
	// before a WebGPU device exists and gives the real uploader stable source data.
	v := append([]Vertex(nil), vertices...)
	i := append([]uint32(nil), indices...)
	return &pendingWebGPUMesh{vertices: v, indices: i}
}

func (WebGPUMeshBackend) Draw(mesh MeshHandle) {
	// No-op until device/surface/pipeline bring-up. This is preferable to calling
	// OpenGL accidentally from a build intended to expose remaining dependencies.
}

type pendingWebGPUMesh struct {
	vertices []Vertex
	indices  []uint32
}

func (m *pendingWebGPUMesh) Draw() {}

func (m *pendingWebGPUMesh) Unload() {
	m.vertices = nil
	m.indices = nil
}

func EnableExperimentalWebGPU() {
	SetMeshBackend(WebGPUMeshBackend{})
}
