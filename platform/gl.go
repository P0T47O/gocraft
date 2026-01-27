package platform

import (
	"log"
	"math"
	"syscall"
	"unsafe"
)

var (
	opengl32 = syscall.NewLazyDLL("opengl32.dll")

	// Standard 1.1 functions
	wglGetProcAddress = opengl32.NewProc("wglGetProcAddress")
	glDrawElements    = opengl32.NewProc("glDrawElements")
	glEnable          = opengl32.NewProc("glEnable")
	glDisable         = opengl32.NewProc("glDisable")
	glBindTexture     = opengl32.NewProc("glBindTexture")
	glPolygonOffset   = opengl32.NewProc("glPolygonOffset")

	// Extensions (Loaded at Runtime)
	glGenBuffers              uintptr
	glBindBuffer              uintptr
	glBufferData              uintptr
	glDeleteBuffers           uintptr
	glGenVertexArrays         uintptr
	glBindVertexArray         uintptr
	glDeleteVertexArrays      uintptr
	glEnableVertexAttribArray uintptr
	glVertexAttribPointer     uintptr
	glUseProgram              uintptr
	glGetUniformLocation      uintptr
	glUniformMatrix4fv        uintptr
	glActiveTexture           uintptr
	glUniform4f               uintptr
	glUniform3f               uintptr
	glUniform1f               uintptr
	glUniform1i               uintptr
)

// Constants
const (
	GL_ARRAY_BUFFER         = 0x8892
	GL_ELEMENT_ARRAY_BUFFER = 0x8893
	GL_STATIC_DRAW          = 0x88E4
	GL_FLOAT                = 0x1406
	GL_UNSIGNED_INT         = 0x1405
	GL_TRIANGLES            = 0x0004
	GL_TEXTURE_2D           = 0x0DE1
	GL_TEXTURE0             = 0x84C0
	GL_POLYGON_OFFSET_FILL  = 0x8037
)

func InitGL() {
	// Must be called AFTER context creation (Raylib InitWindow)

	glGenBuffers = getProc("glGenBuffers")
	glBindBuffer = getProc("glBindBuffer")
	glBufferData = getProc("glBufferData")
	glDeleteBuffers = getProc("glDeleteBuffers")
	glGenVertexArrays = getProc("glGenVertexArrays")
	glBindVertexArray = getProc("glBindVertexArray")
	glDeleteVertexArrays = getProc("glDeleteVertexArrays")
	glEnableVertexAttribArray = getProc("glEnableVertexAttribArray")
	glVertexAttribPointer = getProc("glVertexAttribPointer")
	glUseProgram = getProc("glUseProgram")
	glGetUniformLocation = getProc("glGetUniformLocation")
	glUniformMatrix4fv = getProc("glUniformMatrix4fv")
	glActiveTexture = getProc("glActiveTexture")
	glUniform4f = getProc("glUniform4f")
	glUniform3f = getProc("glUniform3f")
	glUniform1f = getProc("glUniform1f")
	glUniform1i = getProc("glUniform1i")

	log.Println("PureGL: Functions loaded successfully")
}

func getProc(name string) uintptr {
	cname, _ := syscall.BytePtrFromString(name)
	ret, _, _ := wglGetProcAddress.Call(uintptr(unsafe.Pointer(cname)))
	if ret == 0 {
		log.Fatalf("Failed to load GL function: %s", name)
	}
	return ret
}

// Wrappers
func GenBuffer() uint32 {
	var id uint32
	syscall.Syscall(glGenBuffers, 2, 1, uintptr(unsafe.Pointer(&id)), 0)
	return id
}

func BindBuffer(target uint32, buffer uint32) {
	syscall.Syscall(glBindBuffer, 2, uintptr(target), uintptr(buffer), 0)
}

func BufferData(target uint32, size int, data unsafe.Pointer, usage uint32) {
	syscall.Syscall6(glBufferData, 4, uintptr(target), uintptr(size), uintptr(data), uintptr(usage), 0, 0)
}

func GenVertexArray() uint32 {
	var id uint32
	syscall.Syscall(glGenVertexArrays, 2, 1, uintptr(unsafe.Pointer(&id)), 0)
	return id
}

func BindVertexArray(array uint32) {
	syscall.Syscall(glBindVertexArray, 1, uintptr(array), 0, 0)
}

func EnableVertexAttribArray(index uint32) {
	syscall.Syscall(glEnableVertexAttribArray, 1, uintptr(index), 0, 0)
}

func VertexAttribPointer(index uint32, size int32, typeEnum uint32, normalized bool, stride int32, offset uintptr) {
	norm := uintptr(0)
	if normalized {
		norm = 1
	}
	syscall.Syscall6(glVertexAttribPointer, 6, uintptr(index), uintptr(size), uintptr(typeEnum), norm, uintptr(stride), offset)
}

func DrawElements(mode uint32, count int32, typeEnum uint32, indices uintptr) {
	glDrawElements.Call(uintptr(mode), uintptr(count), uintptr(typeEnum), indices)
}

func DeleteBuffer(id uint32) {
	syscall.Syscall(glDeleteBuffers, 2, 1, uintptr(unsafe.Pointer(&id)), 0)
}

func DeleteVertexArray(id uint32) {
	syscall.Syscall(glDeleteVertexArrays, 2, 1, uintptr(unsafe.Pointer(&id)), 0)
}

func UseProgram(program uint32) {
	syscall.Syscall(glUseProgram, 1, uintptr(program), 0, 0)
}

func UniformMatrix4fv(location int32, count int32, transpose bool, value *float32) {
	trans := uintptr(0)
	if transpose {
		trans = 1
	}
	syscall.Syscall6(glUniformMatrix4fv, 4, uintptr(location), uintptr(count), trans, uintptr(unsafe.Pointer(value)), 0, 0)
}

func GetUniformLocation(program uint32, name string) int32 {
	cname, _ := syscall.BytePtrFromString(name)
	ret, _, _ := syscall.Syscall(glGetUniformLocation, 2, uintptr(program), uintptr(unsafe.Pointer(cname)), 0)
	return int32(ret)
}

func ActiveTexture(texture uint32) {
	syscall.Syscall(glActiveTexture, 1, uintptr(texture), 0, 0)
}

func Uniform4f(location int32, v0, v1, v2, v3 float32) {
	syscall.Syscall6(glUniform4f, 5, uintptr(location), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), uintptr(math.Float32bits(v3)), 0)
}

func Uniform3f(location int32, v0, v1, v2 float32) {
	syscall.Syscall6(glUniform3f, 4, uintptr(location), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), 0, 0)
}

func Uniform1f(location int32, v0 float32) {
	syscall.Syscall(glUniform1f, 2, uintptr(location), uintptr(math.Float32bits(v0)), 0)
}

func Uniform1i(location int32, v0 int32) {
	syscall.Syscall(glUniform1i, 2, uintptr(location), uintptr(v0), 0)
}

func BindTexture(target uint32, texture uint32) {
	glBindTexture.Call(uintptr(target), uintptr(texture))
}

func PolygonOffset(factor float32, units float32) {
	glPolygonOffset.Call(uintptr(math.Float32bits(factor)), uintptr(math.Float32bits(units)))
}

func Enable(cap uint32) {
	glEnable.Call(uintptr(cap))
}

func Disable(cap uint32) {
	glDisable.Call(uintptr(cap))
}
