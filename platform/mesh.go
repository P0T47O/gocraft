package platform

var ActiveMeshCount int64

// Vertex is the CPU mesher's 36-byte layout. The production backend packs it
// into 24-byte GPU vertices; diagnostic pipelines can retain the full layout.
type Vertex struct {
	Position [3]float32
	Texcoord [2]float32
	Color    [4]uint8
	Normal   [3]float32
}

func UploadMesh(vertices []Vertex, indices []uint32) MeshHandle {
	return UploadRenderMesh(vertices, indices)
}
