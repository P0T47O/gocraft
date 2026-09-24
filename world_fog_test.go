package main

import "testing"

func TestWorldFogRange(t *testing.T) {
	for _, radius := range []int{2, 4, 8, 16, 24, 32} {
		start, end := worldFogRange(radius)
		distance := float32(radius * chunkWidth)
		if start < distance*0.69 || end <= start || end >= distance {
			t.Fatalf("radius %d: invalid fog interval [%f, %f]", radius, start, end)
		}
	}
	start, end := worldFogRange(worldRenderRadius)
	if start < 310 || end != 360 {
		t.Fatalf("default fog should leave middle distance clear: [%f, %f]", start, end)
	}
	start, end = worldFogRange(32)
	if start < 420 || end != 488 {
		t.Fatalf("32 chunks should fade only near the boundary: [%f, %f]", start, end)
	}
}
