package platform

import (
	"sync/atomic"
	"unsafe"
)

var ActiveMeshCount int64

// Vertex is the backend-neutral 36-byte chunk vertex layout. Backends must
// preserve these semantics so the CPU chunk mesher does not depend on a GPU API.
type Vertex struct {
	Position [3]float32
	Texcoord [2]float32
	Color    [4]uint8
	Normal   [3]float32
}

// Mesh is the OpenGL implementation of MeshHandle. Its GL object identities
// remain private so callers can hold the backend-neutral MeshHandle instead.
type Mesh struct {
	vao        uint32
	vbo        uint32
	ebo        uint32
	indexCount int32
}

// UploadMesh is kept as the compatibility entry point used by the current
// chunk mesher and previews. It now routes through the selected MeshBackend,
// so existing CPU meshing code does not need to know whether OpenGL or WebGPU
// owns the resulting GPU buffers.
func UploadMesh(vertices []Vertex, indices []uint32) MeshHandle {
	return UploadRenderMesh(vertices, indices)
}

// uploadOpenGLMesh is the concrete OpenGL upload used only by
// OpenGLMeshBackend. Keeping it private prevents the renderer-neutral path from
// accidentally bypassing backend selection again.
func uploadOpenGLMesh(vertices []Vertex, indices []uint32) *Mesh {
	InitGLOnce()

	vao := GenVertexArray()
	BindVertexArray(vao)

	vbo := GenBuffer()
	BindBuffer(GL_ARRAY_BUFFER, vbo)
	if len(vertices) > 0 {
		size := len(vertices) * int(unsafe.Sizeof(Vertex{}))
		BufferData(GL_ARRAY_BUFFER, size, unsafe.Pointer(&vertices[0]), GL_STATIC_DRAW)
	}

	ebo := GenBuffer()
	BindBuffer(GL_ELEMENT_ARRAY_BUFFER, ebo)
	if len(indices) > 0 {
		size := len(indices) * 4
		BufferData(GL_ELEMENT_ARRAY_BUFFER, size, unsafe.Pointer(&indices[0]), GL_STATIC_DRAW)
	}

	var vertex Vertex
	stride := int32(unsafe.Sizeof(vertex))
	EnableVertexAttribArray(0)
	VertexAttribPointer(0, 3, GL_FLOAT, false, stride, unsafe.Offsetof(vertex.Position))
	EnableVertexAttribArray(1)
	VertexAttribPointer(1, 2, GL_FLOAT, false, stride, unsafe.Offsetof(vertex.Texcoord))
	EnableVertexAttribArray(3)
	VertexAttribPointer(3, 4, GL_UNSIGNED_BYTE, true, stride, unsafe.Offsetof(vertex.Color))
	EnableVertexAttribArray(2)
	VertexAttribPointer(2, 3, GL_FLOAT, false, stride, unsafe.Offsetof(vertex.Normal))

	BindVertexArray(0)
	atomic.AddInt64(&ActiveMeshCount, 1)
	return &Mesh{vao: vao, vbo: vbo, ebo: ebo, indexCount: int32(len(indices))}
}

func (m *Mesh) Draw() {
	if m == nil || m.vao == 0 {
		return
	}
	BindVertexArray(m.vao)
	DrawElements(GL_TRIANGLES, m.indexCount, GL_UNSIGNED_INT, 0)
	BindVertexArray(0)
}

func (m *Mesh) IndexCount() int32 {
	if m == nil {
		return 0
	}
	return m.indexCount
}

func (m *Mesh) Unload() {
	if m == nil {
		return
	}
	hadResource := m.vao != 0 || m.vbo != 0 || m.ebo != 0
	if m.vao != 0 {
		DeleteVertexArray(m.vao)
		m.vao = 0
	}
	if m.vbo != 0 {
		DeleteBuffer(m.vbo)
		m.vbo = 0
	}
	if m.ebo != 0 {
		DeleteBuffer(m.ebo)
		m.ebo = 0
	}
	if hadResource {
		atomic.AddInt64(&ActiveMeshCount, -1)
	}
}

var glInit = false

func InitGLOnce() {
	if !glInit {
		InitGL()
		glInit = true
	}
}
