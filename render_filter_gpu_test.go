package main

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Opt-in driver readback, without reading/writing personal settings or saves.
func TestTextureFilterGPU(t *testing.T) {
	if os.Getenv("GOCRAFT_FILTER_GPU_TEST") != "1" {
		t.Skip("opt-in OpenGL test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 240, "Texture filter test")
	defer rl.CloseWindow()
	oldSettings, oldLimit := currentSettings, textureAnisotropyLimit
	defer func() { currentSettings, textureAnisotropyLimit = oldSettings, oldLimit }()
	textureAnisotropyLimit = 0
	currentSettings = &GameSettings{}
	img := rl.GenImageColor(256, 256, rl.White)
	texture := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	defer rl.UnloadTexture(texture)
	dll := syscall.NewLazyDLL("opengl32.dll")
	bind := dll.NewProc("glBindTexture")
	getInt := dll.NewProc("glGetTexParameteriv")
	getFloat := dll.NewProc("glGetTexParameterfv")
	getError := dll.NewProc("glGetError")
	for {
		e, _, _ := getError.Call()
		if e == 0 {
			break
		}
	}
	t.Logf("Hardware AF limit: %dx", supportedAnisotropy())
	for _, mipmaps := range []bool{true, false, true} {
		for _, af := range []int{1, 2, 4, 8, 16, 1} {
			currentSettings.Mipmaps, currentSettings.Anisotropy = mipmaps, af
			configureWorldTexture(&texture, true)
			bind.Call(0x0DE1, uintptr(texture.ID))
			var mag, minFilter, maxLevel int32
			getInt.Call(0x0DE1, 0x2800, uintptr(unsafe.Pointer(&mag)))
			getInt.Call(0x0DE1, 0x2801, uintptr(unsafe.Pointer(&minFilter)))
			getInt.Call(0x0DE1, 0x813D, uintptr(unsafe.Pointer(&maxLevel)))
			effective := min(af, supportedAnisotropy())
			wantMin := int32(rl.TextureFilterNearest)
			if mipmaps {
				wantMin = rl.TextureFilterNearestMipLinear
			}
			if effective > 1 {
				if mipmaps {
					wantMin = rl.TextureFilterMipLinear
				} else {
					wantMin = rl.TextureFilterLinear
				}
			}
			if mag != rl.TextureFilterNearest || minFilter != wantMin || maxLevel != int32(safeAtlasMipLevel(effective)) {
				t.Fatalf("mips=%v AF=%d: unexpected sampler: %d %d %d, want min=%d", mipmaps, af, mag, minFilter, maxLevel, wantMin)
			}
			if supportedAnisotropy() > 1 {
				var actual float32
				getFloat.Call(0x0DE1, 0x84FE, uintptr(unsafe.Pointer(&actual)))
				if actual != float32(effective) {
					t.Fatalf("AF got %v want %v", actual, effective)
				}
			}
			bind.Call(0x0DE1, 0)
			if e, _, _ := getError.Call(); e != 0 {
				t.Fatalf("GL error: %x", e)
			}
		}
	}
}
