package platform

import (
	"testing"
	"unsafe"
)

func TestVertexLayout(t *testing.T) {
	var vertex Vertex
	if got := unsafe.Sizeof(vertex); got != 36 {
		t.Fatalf("vertex stride = %d, want 36", got)
	}
	offsets := []uintptr{
		unsafe.Offsetof(vertex.Position), unsafe.Offsetof(vertex.Texcoord),
		unsafe.Offsetof(vertex.Color), unsafe.Offsetof(vertex.Normal),
	}
	for i, want := range []uintptr{0, 12, 20, 24} {
		if offsets[i] != want {
			t.Fatalf("attribute %d offset = %d, want %d", i, offsets[i], want)
		}
	}
}
