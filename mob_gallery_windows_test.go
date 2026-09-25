//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/wgpu"
)

func TestWebGPUMobGallery(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("opt-in GPU mob gallery")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	old := remoteEntities
	defer func() { remoteEntities = old }()
	remoteEntities = map[string]*RemoteEntity{}
	for i, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
		remoteEntities[kind] = &RemoteEntity{ID: kind, Type: EntityPig, MobKind: kind, MobHealth: mobContent.Definitions[kind].Health, X: float64(i*2 - 3), Y: 0, Z: 0, TX: float64(i*2 - 3)}
	}
	const width, height = 1280, 720
	if err := r.prepareSurface(width, height); err != nil {
		t.Fatal(err)
	}
	camera := webGPUCamera{Position: webGPUVec3{X: 0, Y: 2.3, Z: 8}, Target: webGPUVec3{X: 0, Y: .85, Z: 0}, Up: webGPUVec3{Y: 1}, Fovy: 50}
	if err := r.updateSceneCameraAtTime(camera, 6000); err != nil {
		t.Fatal(err)
	}
	entities, err := ensureWebGPUEntityRenderer(r)
	if err != nil {
		t.Fatal(err)
	}
	img := captureNativePreview(t, r, width, height, func(pass *wgpu.RenderPassEncoder) error {
		return entities.Draw(pass, r, nil, 0)
	})
	path := filepath.Join("work", "mob-gallery.png")
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	saveNativePreview(t, img, path)
	t.Log(path)
}
