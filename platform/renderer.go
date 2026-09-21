package platform

// MeshHandle is GPU geometry owned by its backend and render thread.
// Drawing requires an explicit WebGPU render pass, not a global Draw call.
type MeshHandle interface {
	Unload()
	IndexCount() int32
}
type MeshBackend interface {
	Upload(vertices []Vertex, indices []uint32) MeshHandle
}

// No implicit graphics backend: uploads before device initialization are bugs.
var meshBackend MeshBackend

func SetMeshBackend(backend MeshBackend) { meshBackend = backend }
func UploadRenderMesh(vertices []Vertex, indices []uint32) MeshHandle {
	if meshBackend == nil {
		panic("mesh upload without an active renderer")
	}
	return meshBackend.Upload(vertices, indices)
}
