package platform

// CompactVertex omits the normal unused by the live baked-light world shader.
// Keep Vertex unchanged for CPU meshing and diagnostic pipelines using normals.
type CompactVertex struct {
	Position [3]float32
	Texcoord [2]float32
	Color    [4]uint8
}

func PackCompactVertices(dst []CompactVertex, src []Vertex) []CompactVertex {
	if cap(dst) < len(src) {
		dst = make([]CompactVertex, len(src))
	} else {
		dst = dst[:len(src)]
	}
	for i, v := range src {
		dst[i] = CompactVertex{v.Position, v.Texcoord, v.Color}
	}
	return dst
}
