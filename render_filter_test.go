package main

import "testing"

func TestAtlasMipIsolation(t *testing.T) {
	for level := 0; level <= atlasMaxMipLevel; level++ {
		scale := 1 << level
		if atlasCellSize%scale != 0 || atlasPadding%scale != 0 || atlasTileSize%scale != 0 {
			t.Fatal("unaligned mip cells")
		}
		if atlasPadding/scale < 1 || (atlasCellSize-atlasPadding-atlasTileSize)/scale < 1 {
			t.Fatal("missing mip gutter")
		}
	}
	for p := 0; p < atlasCellSize; p++ {
		got := atlasSourceCoordinate(p)
		if got < 0 || got >= atlasTileSize {
			t.Fatal("out of tile read")
		}
		if p < atlasPadding && got != 0 {
			t.Fatal("left edge not extruded")
		}
		if p >= atlasPadding+atlasTileSize && got != atlasTileSize-1 {
			t.Fatal("right edge not extruded")
		}
		if p >= atlasPadding && p < atlasPadding+atlasTileSize && got != p-atlasPadding {
			t.Fatal("interior changed")
		}
	}
}
