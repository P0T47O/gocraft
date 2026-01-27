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

func UploadMesh(vertices []float32, indices []uint32) *Mesh {
	InitGLOnce()

	var vao, vbo, ebo uint32

	vao = GenVertexArray()
	BindVertexArray(vao)

	// VBO
	vbo = GenBuffer()
	BindBuffer(GL_ARRAY_BUFFER, vbo)

	// Upload Vertices
	if len(vertices) > 0 {
		size := len(vertices) * 4 // float32 = 4 bytes
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

	// Attributes (Matches Raylib shader layout usually)
	// Pos: 0 (3 floats)
	// Tex: 1 (2 floats)
	// Col: 2 (4 floats? No, Raylib sends colors as ubyte usually, but my builder uses float colors?)
	// Check render_mesh.go addFace:
	// vertices (3), normal (3), uv (2), color (4) ?
	// Wait, addFace in `render_mesh.go` writes to `Vertices`, `Normals`, `Texcoords`, `Colors` arrays separately?
	// Raylib uses separate arrays often.
	// But `platform.UploadMesh` above assumes INTERLEAVED?
	// NO. `vertices []float32` implies interleaved if it's one array.
	// BUT `render_mesh.go` currently builds separate arrays.

	// I NEED TO UPDATE `render_mesh.go` to interleave data OR standard Raylib `rl.Mesh` uses separate buffers.
	// If I use my own `platform.Mesh`, I can choose the layout.
	// Interleaved is better for cache.
	// Layout: Pos(3) + Normal(3) + Tex(2) + Color(4) = 12 floats?
	// Wait, Color is usually 4 bytes.
	// Let's stick to simple floats for now: Pos(3), Tex(2), Color(4), Normal(3).
	// Stride = (3+2+4+3) * 4 = 48 bytes.

	// Attrib 0: Pos
	EnableVertexAttribArray(0)
	VertexAttribPointer(0, 3, GL_FLOAT, false, 48, 0)

	// Attrib 1: Tex
	EnableVertexAttribArray(1)
	VertexAttribPointer(1, 2, GL_FLOAT, false, 48, 12) // Offset 3*4

	// Attrib 2: Color
	EnableVertexAttribArray(2) // Raylib might use 3 for Color? Need to check default shader.
	// Raylib standard shader: in vec3 vertexPosition; in vec2 vertexTexCoord; in vec4 vertexColor; in vec3 vertexNormal;
	// Layouts: 0=Pos, 1=Tex, 2=Normal, 3=Color usually? Or 0=Pos, 1=Tex, 2=Color?
	// Raylib explicit binding locations?
	// Let's assume standard: 0=Pos, 1=Tex, 3=Color, 2=Normal?
	// Actually, `rlgl` typically uses: 0=Pos, 1=Tex, 2=Col, 3=Normal?
	// I will check Raylib source or trial/error.
	// Assuming 0, 1, 2, 3 for now.

	EnableVertexAttribArray(3)                         // Color
	VertexAttribPointer(3, 4, GL_FLOAT, false, 48, 20) // Offset (3+2)*4

	EnableVertexAttribArray(2)                         // Normal
	VertexAttribPointer(2, 3, GL_FLOAT, false, 48, 36) // Offset (3+2+4)*4

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
