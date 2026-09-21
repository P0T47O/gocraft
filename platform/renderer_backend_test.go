package platform

import "testing"

type recordingMeshBackend struct {
	uploads     int
	vertexCount int
	indexCount  int
}

func (b *recordingMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	b.uploads++
	b.vertexCount = len(vertices)
	b.indexCount = len(indices)
	return &recordingMeshHandle{indexCount: int32(len(indices))}
}

type recordingMeshHandle struct {
	indexCount int32
}

func (m *recordingMeshHandle) Unload()           {}
func (m *recordingMeshHandle) IndexCount() int32 { return m.indexCount }

func TestUploadMeshRoutesThroughSelectedBackend(t *testing.T) {
	previous := meshBackend
	defer func() { meshBackend = previous }()

	backend := &recordingMeshBackend{}
	SetMeshBackend(backend)

	mesh := UploadMesh(
		[]Vertex{{Position: [3]float32{1, 2, 3}}},
		[]uint32{0, 0, 0},
	)
	if mesh == nil {
		t.Fatal("UploadMesh returned nil handle")
	}
	if backend.uploads != 1 {
		t.Fatalf("backend uploads = %d, want 1", backend.uploads)
	}
	if backend.vertexCount != 1 || backend.indexCount != 3 {
		t.Fatalf("backend saw %d vertices / %d indices, want 1 / 3", backend.vertexCount, backend.indexCount)
	}
	if mesh.IndexCount() != 3 {
		t.Fatalf("mesh index count = %d, want 3", mesh.IndexCount())
	}
}
