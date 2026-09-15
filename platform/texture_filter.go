package platform

import (
	"github.com/ebitengine/purego"
	"unsafe"
)

// MaxTextureAnisotropy requires the render thread and a current GL 3.3 context.
// Check extensions before querying the enum, so unsupported GPUs stay error-free.
func MaxTextureAnisotropy() int {
	var count int32
	opengl32.NewProc("glGetIntegerv").Call(0x821D, uintptr(unsafe.Pointer(&count)))
	var getString func(uint32, uint32) string
	purego.RegisterFunc(&getString, getProc("glGetStringi"))
	for i := int32(0); i < count; i++ {
		name := getString(0x1F03, uint32(i))
		if name == "GL_EXT_texture_filter_anisotropic" || name == "GL_ARB_texture_filter_anisotropic" {
			var limit float32 = 1
			opengl32.NewProc("glGetFloatv").Call(0x84FF, uintptr(unsafe.Pointer(&limit)))
			return max(1, int(limit))
		}
	}
	return 1
}
