package main

import (
	"encoding/json"
	"testing"
)

func TestTextureFilterSettingsRoundTrip(t *testing.T) {
	s := GameSettings{Mipmaps: true, Anisotropy: 8, MSAASamples: 4}
	if err := json.Unmarshal([]byte(`{"PlayerName":"Old"}`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Mipmaps || s.Anisotropy != 8 || s.MSAASamples != 4 {
		t.Fatal("missing fields lost defaults")
	}
	s.Mipmaps, s.Anisotropy = false, 16
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored GameSettings
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Mipmaps || restored.Anisotropy != 16 || restored.MSAASamples != 4 {
		t.Fatal("filter settings lost")
	}
}

func TestMSAASampleNormalization(t *testing.T) {
	for input, expected := range map[int]int{-1: 4, 0: 4, 1: 1, 2: 4, 4: 4, 8: 4} {
		if got := normalizeMSAASamples(input); got != expected {
			t.Fatalf("%d samples: got %d, want %d", input, got, expected)
		}
	}
}

func TestAnisotropyNormalization(t *testing.T) {
	for _, tc := range [][2]int{{-1, 1}, {0, 1}, {1, 1}, {2, 2}, {3, 2}, {7, 4}, {8, 8}, {16, 16}, {100, 16}} {
		if got := normalizeAnisotropy(tc[0]); got != tc[1] {
			t.Fatalf("%d: got %d want %d", tc[0], got, tc[1])
		}
	}
}

func TestAnisotropicAtlasGutters(t *testing.T) {
	if safeAtlasMipLevel(8) != atlasMaxMipLevel {
		t.Fatal("default AF must retain all mip levels")
	}
	for _, af := range []int{1, 2, 4, 8, 16} {
		level := safeAtlasMipLevel(af)
		if level < 0 || level > atlasMaxMipLevel {
			t.Fatal("invalid level", level)
		}
		if atlasPadding/(1<<level) < af/2+1 {
			t.Fatal("unsafe footprint", af, level)
		}
	}
}

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
