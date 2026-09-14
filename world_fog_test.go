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
	if start < 260 || end != 360 {
		t.Fatalf("default fog should leave middle distance clear: [%f, %f]", start, end)
	}
}
