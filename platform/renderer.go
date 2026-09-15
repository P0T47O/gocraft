package platform

// MeshBackend is the first narrow renderer seam used by the WebGPU experiment.
// It deliberately covers only GPU mesh lifetime: frame, texture and pipeline
// abstractions will be introduced when the WebGPU bring-up actually needs them.
type MeshBackend interface {
	Upload(vertices []Vertex, indices []uint32) MeshHandle
}

// MeshHandle is backend-owned GPU geometry. GPU operations must remain on the
// render thread, matching the engine's existing ownership rules.
type MeshHandle interface {
	Draw()
	Unload()
}

// OpenGLMeshBackend adapts the existing renderer to the backend-neutral seam.
// Keeping this tiny is intentional: GoCraft should not invent a general-purpose
// graphics API just to support an experiment.
type OpenGLMeshBackend struct{}

func (OpenGLMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	return UploadMesh(vertices, indices)
}

var meshBackend MeshBackend = OpenGLMeshBackend{}

// SetMeshBackend is intended for renderer initialization before any world mesh
// is uploaded. Switching it while a world is active would mix GPU resources
// owned by different backends and is therefore unsupported.
func SetMeshBackend(backend MeshBackend) {
	if backend == nil {
		meshBackend = OpenGLMeshBackend{}
		return
	}
	meshBackend = backend
}

func UploadRenderMesh(vertices []Vertex, indices []uint32) MeshHandle {
	return meshBackend.Upload(vertices, indices)
}
