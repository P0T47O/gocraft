package main

import (
	"bytes"
	"testing"
)

func TestAnimationMipCacheEquivalentAndBounded(t *testing.T) {
	frames := [][]byte{make([]byte, 4*4*4), make([]byte, 4*4*4)}
	for f := range frames {
		for i := range frames[f] {
			frames[f][i] = byte(i*31 + f*83)
		}
	}
	base := webGPUAtlasAnimation{frames: frames, width: 4, height: 4, bytesPerRow: 16}
	frameBytes := 0
	for level := 0; level <= atlasMaxMipLevel; level++ {
		size := max(1, atlasCellSize>>level)
		frameBytes += size * size * 4
	}
	animations := []webGPUAtlasAnimation{base, base}
	prepareWebGPUAnimationMips(animations, 2*frameBytes)
	if len(animations[0].cachedMips) != 2 || len(animations[1].cachedMips) != 0 {
		t.Fatal("cache exceeded shared budget")
	}
	for cycle := 0; cycle < 3; cycle++ {
		for frame := range frames {
			want, got := base.mipFrame(frame), animations[0].mipFrame(frame)
			for level := range want {
				if !bytes.Equal(want[level].pixels, got[level].pixels) {
					t.Fatalf("cycle %d frame %d mip %d differs", cycle, frame, level)
				}
			}
		}
	}
	if &animations[0].mipFrame(0)[0].pixels[0] == &animations[0].mipFrame(1)[0].pixels[0] {
		t.Fatal("frames alias mutable scratch")
	}
}
