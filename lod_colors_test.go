package main

import (
	"image"
	"image/color"
	"testing"
)

func TestLODTextureAverageIgnoresTransparentPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, B: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{R: 80, G: 80, B: 80, A: 128})
	if got := lodTextureAverage(img); got != ([4]uint8{187, 187, 187, 255}) {
		t.Fatalf("weighted texture average=%v", got)
	}
}

func TestLODSnowUsesLoadedTexturePalette(t *testing.T) {
	initBlockRegistry()
	previous := activeLODMaterialPalette.Load()
	defer activeLODMaterialPalette.Store(previous)
	want := [4]uint8{248, 250, 251, 255}
	publishLODMaterialPalette(map[string][4]uint8{"textures/block/snow.png": want})
	if got := lodSurfaceColorAt(12345, 10, 20, blockSnow); got != want {
		t.Fatalf("snow retained the hard-coded palette: got %v, want %v", got, want)
	}
}
