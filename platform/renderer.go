package platform

// MeshHandle is backend-owned GPU geometry. GPU operations must remain on the
// render thread, matching the engine's existing ownership rules.
type MeshHandle interface {
	Draw()
	Unload()
	IndexCount() int32
}

// MeshBackend is the first narrow renderer seam used by the WebGPU experiment.
// Draw receives renderer state that the current OpenGL path needs; a WebGPU
// backend can instead interpret it as pipeline/bind-group state.
type MeshBackend interface {
	Upload(vertices []Vertex, indices []uint32) MeshHandle
	Draw(mesh MeshHandle)
}

type OpenGLMeshBackend struct{}

func (OpenGLMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	return uploadOpenGLMesh(vertices, indices)
}

func (OpenGLMeshBackend) Draw(mesh MeshHandle) {
	if mesh != nil {
		mesh.Draw()
	}
}

var meshBackend MeshBackend = OpenGLMeshBackend{}

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

func DrawRenderMesh(mesh MeshHandle) {
	meshBackend.Draw(mesh)
}
