//go:build windows

package main

import (
	"os"
	"testing"
)

// Keep the existing opt-in entry, now targeting the production WebGPU sampler,
// mip views and GPU pixel readback rather than removed OpenGL state.
func TestTextureFilterGPU(t *testing.T) {
	if os.Getenv("GOCRAFT_FILTER_GPU_TEST") != "1" {
		t.Skip("opt-in WebGPU filter test")
	}
	t.Setenv("GOCRAFT_WEBGPU_REGRESSION", "1")
	t.Run("mip-stability", TestWebGPUMipStabilityGPU)
	t.Run("sampler-and-UI-isolation", TestWebGPUUIUploadIsolationGPU)
}
