package main

import (
	"encoding/json"
	"testing"
)

func TestRenderDistanceSettings(t *testing.T) {
	for _, tc := range [][2]int{{0, 24}, {-10, 8}, {8, 8}, {24, 24}, {32, 32}, {200, 32}} {
		if got := clampRenderDistance(tc[0]); got != tc[1] {
			t.Fatalf("%d => %d", tc[0], got)
		}
	}
	s := GameSettings{RenderDistance: defaultRenderDistance}
	if err := json.Unmarshal([]byte(`{"PlayerName":"OldSettings"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.RenderDistance != 24 {
		t.Fatal("missing field lost default")
	}
	s.RenderDistance = 32
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored GameSettings
	if err = json.Unmarshal(b, &restored); err != nil || restored.RenderDistance != 32 {
		t.Fatal("distance round trip")
	}
	start, end := worldFogRange(32)
	if start <= 0 || end >= 32*chunkWidth || start >= end {
		t.Fatal("invalid fog at maximum distance")
	}
}
