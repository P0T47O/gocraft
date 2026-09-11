package platform

import (
	"sync/atomic"
	"unsafe"
)

var ActiveMeshCount int64

type Mesh struct {
	VAO        uint32
	VBO        uint32
	EBO        uint32
	IndexCount int32
}

// Vertex is the 36-byte GPU layout. Color is normalized by GL on fetch,
// preserving byte color precision while reducing vertex bandwidth by 25%.
type Vertex struct {
	Position [3]float32
	Texcoord [2]float32
	Color    [4]uint8
	Normal   [3]float32
}

func UploadMesh(vertices []Vertex, indices []uint32) *Mesh {
	InitGLOnce()

	var vao, vbo, ebo uint32

	vao = GenVertexArray()
	BindVertexArray(vao)

	// VBO
	vbo = GenBuffer()
	BindBuffer(GL_ARRAY_BUFFER, vbo)

	// Upload Vertices
	if len(vertices) > 0 {
		size := len(vertices) * int(unsafe.Sizeof(Vertex{}))
		BufferData(GL_ARRAY_BUFFER, size, unsafe.Pointer(&vertices[0]), GL_STATIC_DRAW)
	}

	// EBO
	ebo = GenBuffer()
	BindBuffer(GL_ELEMENT_ARRAY_BUFFER, ebo)

	// Upload Indices
	if len(indices) > 0 {
		size := len(indices) * 4 // uint32 = 4 bytes
		BufferData(GL_ELEMENT_ARRAY_BUFFER, size, unsafe.Pointer(&indices[0]), GL_STATIC_DRAW)
	}

	// Raylib shader locations: position=0, texcoord=1, normal=2, color=3.
	// Derive offsets from the Go layout so the upload and attribute declarations
	// cannot silently disagree.
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

	return &Mesh{
		VAO:        vao,
		VBO:        vbo,
		EBO:        ebo,
		IndexCount: int32(len(indices)),
	}
}

func (m *Mesh) Draw() {
	if m.VAO == 0 {
		return
	}
	BindVertexArray(m.VAO)
	DrawElements(GL_TRIANGLES, m.IndexCount, GL_UNSIGNED_INT, 0)
	BindVertexArray(0)
}

func (m *Mesh) Unload() {
	if m.VAO != 0 {
		DeleteVertexArray(m.VAO)
		m.VAO = 0
	}
	if m.VBO != 0 {
		DeleteBuffer(m.VBO)
		m.VBO = 0
	}
	if m.EBO != 0 {
		DeleteBuffer(m.EBO)
		m.EBO = 0
	}
	atomic.AddInt64(&ActiveMeshCount, -1)
}

var glInit = false

func InitGLOnce() {
	if !glInit {
		InitGL()
		glInit = true
	}
}
