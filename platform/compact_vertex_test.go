package platform

import (
	"testing"
	"unsafe"
)

func TestCompactVertexLayout(t *testing.T) {
	v := CompactVertex{}
	if unsafe.Sizeof(v) != 24 || unsafe.Offsetof(v.Color) != 20 || unsafe.Offsetof(v.Texcoord) != 12 {
		t.Fatal("shader layout mismatch")
	}
	src := []Vertex{{Position: [3]float32{1, 2, 3}, Texcoord: [2]float32{.25, .5}, Color: [4]byte{1, 2, 3, 4}, Normal: [3]float32{0, 1, 0}}}
	dst := PackCompactVertices(make([]CompactVertex, 0, 2), src)
	if dst[0].Position != src[0].Position || dst[0].Texcoord != src[0].Texcoord || dst[0].Color != src[0].Color {
		t.Fatal("vertex changed")
	}
	ptr := &dst[0]
	dst = PackCompactVertices(dst, src)
	if ptr != &dst[0] {
		t.Fatal("scratch storage not reused")
	}
}
