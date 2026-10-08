//go:build windows

package main

import (
	"bytes"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
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

// Render every registered mob from three directions with the production atlas
// and entity pipeline. Each image keeps the same left-to-right species order so
// UV mistakes and missing faces are easy to compare without a world save.
func TestWebGPUMobThreeViewGallery(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_MOB_VIEWS") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_MOB_VIEWS=1 to capture all mob views")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	kinds := make([]string, 0, len(mobContent.Definitions))
	for kind := range mobContent.Definitions {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	const width, height = 1280, 720
	const spacing = float32(2.3)
	for i, kind := range kinds {
		x := float64((float32(i) - float32(len(kinds)-1)/2) * spacing)
		remoteEntities[kind] = &RemoteEntity{
			ID: kind, Type: EntityPig, MobKind: kind,
			MobHealth: mobContent.Definitions[kind].Health,
			X:         x, Y: 0, Z: 0, TX: x,
		}
	}
	if err := r.prepareSurface(width, height); err != nil {
		t.Fatal(err)
	}
	distance := max(float32(13), float32(len(kinds))*1.9)
	camera := webGPUCamera{
		Position: webGPUVec3{X: 0, Y: 2.3, Z: distance},
		Target:   webGPUVec3{X: 0, Y: .85, Z: 0},
		Up:       webGPUVec3{Y: 1}, Fovy: 45,
	}
	entities, err := ensureWebGPUEntityRenderer(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []struct {
		name string
		yaw  float32
	}{
		{"front", 0}, {"three-quarter", math.Pi / 4}, {"side", math.Pi / 2}, {"back", math.Pi},
	} {
		for _, e := range remoteEntities {
			e.Yaw = view.yaw
		}
		if err := r.updateSceneCameraAtTime(camera, 6000); err != nil {
			t.Fatal(err)
		}
		img := captureNativePreview(t, r, width, height, func(pass *wgpu.RenderPassEncoder) error {
			return entities.Draw(pass, r, nil, 0)
		})
		assertMobGalleryPopulated(t, img, kinds, view.name)
		path := filepath.Join("work", "mob-views-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("%s: %s; left to right: %v", view.name, path, kinds)
	}
	// The smaller silhouettes in the full gallery hide incorrect face/UV
	// placement. Keep a second, close camera on the three reported species.
	focus := []string{"chicken", "sheep", "spider"}
	remoteEntities = make(map[string]*RemoteEntity, len(focus))
	for i, kind := range focus {
		x := float64((float32(i) - 1) * 2.5)
		remoteEntities[kind] = &RemoteEntity{
			ID: kind, Type: EntityPig, MobKind: kind,
			MobHealth: mobContent.Definitions[kind].Health,
			X:         x, Y: 0, Z: 0, TX: x,
		}
	}
	camera = webGPUCamera{
		Position: webGPUVec3{X: 0, Y: 1.8, Z: 6.5},
		Target:   webGPUVec3{X: 0, Y: .65, Z: 0},
		Up:       webGPUVec3{Y: 1}, Fovy: 45,
	}
	var woollySide []byte
	for _, view := range []struct {
		name string
		yaw  float32
	}{
		{"front", 0}, {"three-quarter", math.Pi / 4}, {"side", math.Pi / 2}, {"back", math.Pi},
	} {
		for _, e := range remoteEntities {
			e.Yaw = view.yaw
		}
		if err := r.updateSceneCameraAtTime(camera, 6000); err != nil {
			t.Fatal(err)
		}
		img := captureNativePreview(t, r, width, height, func(pass *wgpu.RenderPassEncoder) error {
			return entities.Draw(pass, r, nil, 0)
		})
		if view.name == "side" {
			woollySide = append([]byte(nil), img.Pix...)
		}
		assertMobGalleryPopulated(t, img, focus, view.name)
		path := filepath.Join("work", "mob-focus-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("%s close-up: %s; left to right: %v", view.name, path, focus)
	}
	remoteEntities["sheep"].MobSheared = true
	for _, e := range remoteEntities {
		e.Yaw = math.Pi / 2
	}
	if err := r.updateSceneCameraAtTime(camera, 6000); err != nil {
		t.Fatal(err)
	}
	sheared := captureNativePreview(t, r, width, height, func(pass *wgpu.RenderPassEncoder) error {
		return entities.Draw(pass, r, nil, 0)
	})
	if bytes.Equal(woollySide, sheared.Pix) {
		t.Fatal("sheared sheep looks identical to woolly sheep")
	}
	saveNativePreview(t, sheared, filepath.Join("work", "mob-focus-sheared.png"))
}

func assertMobGalleryPopulated(t *testing.T, img *image.RGBA, kinds []string, view string) {
	t.Helper()
	background := img.RGBAAt(0, 0)
	for i, kind := range kinds {
		// The camera is centered on a row with uniformly spaced entities.
		// Count changed pixels in each slot, avoiding the upper sky/other slots.
		left := i * img.Bounds().Dx() / len(kinds)
		right := (i + 1) * img.Bounds().Dx() / len(kinds)
		changed := 0
		for y := img.Bounds().Dy() / 4; y < img.Bounds().Dy()*3/4; y++ {
			for x := left; x < right; x++ {
				if c := img.RGBAAt(x, y); c != background {
					changed++
				}
			}
		}
		if changed < 100 {
			t.Errorf("%s view: %s appears missing or invisible (%d non-background pixels)", view, kind, changed)
		}
	}
}
