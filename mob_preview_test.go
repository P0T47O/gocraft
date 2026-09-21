//go:build windows

package main

import (
	"github.com/gogpu/wgpu"
	"os"
	"testing"
)

func TestMobRenderPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_MOB_PREVIEW") != "1" {
		t.Skip("opt-in native preview")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	for _, state := range []string{"idle", "walk", "hurt", "death"} {
		e := &RemoteEntity{Type: EntityPig, MobKind: "pig", MobHealth: 10}
		switch state {
		case "walk":
			e.AnimBlend = 1
			e.AnimPhase = 1
		case "hurt":
			e.MobHurt = 1
		case "death":
			e.DeathTime = .6
		}
		remoteEntities = map[string]*RemoteEntity{"preview-pig": e}
		image := captureNativePreview(t, r, 1000, 700, func(pass *wgpu.RenderPassEncoder) error {
			if err := r.updateSceneCamera(webGPUCamera{Position: webGPUVec3{X: 2.3, Y: 1.7, Z: 3}, Target: webGPUVec3{Y: .5}, Up: webGPUVec3{Y: 1}, Fovy: 45}); err != nil {
				return err
			}
			renderer, err := ensureWebGPUEntityRenderer(r)
			if err != nil {
				return err
			}
			return renderer.Draw(pass, r, nil, 0)
		})
		visible := 0
		for i := 0; i < len(image.Pix); i += 4 {
			if image.Pix[i] != 0 || image.Pix[i+1] != 0 || image.Pix[i+2] != 0 {
				visible++
			}
		}
		if visible < 100 {
			t.Fatalf("%s mob was not rendered: %d colored pixels", state, visible)
		}
		saveNativePreview(t, image, "work/pig-"+state+".png")
	}
}
