package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestGameplayCameraAim(t *testing.T) {
	for _, direction := range []gameVec3{{X: 1}, {Z: 1}, {X: -2, Y: 1, Z: 3}, {}} {
		c := gameCamera{Position: gameVec3{X: 17, Y: 80, Z: -300}}
		c.Target = gameVec3Add(c.Position, direction)
		s := &InputState{}
		s.InitFromCamera(c)
		origin, ray := s.RayFromCenter(c)
		if origin != c.Position || ray != gameVec3Normalize(direction) {
			t.Fatalf("incorrect camera ray: %v %v", origin, ray)
		}
		if math.IsNaN(float64(s.Yaw)) || math.IsNaN(float64(s.Pitch)) {
			t.Fatal("invalid camera angles")
		}
		if direction != (gameVec3{}) {
			reconstructed := newGameVec3(float32(math.Sin(float64(s.Yaw))*math.Cos(float64(s.Pitch))), float32(math.Sin(float64(s.Pitch))), float32(math.Cos(float64(s.Yaw))*math.Cos(float64(s.Pitch))))
			if gameVec3Length(gameVec3Subtract(reconstructed, ray)) > 1e-5 {
				t.Fatal("camera angles and aiming ray disagree")
			}
		}
	}
}

func TestPlayerSaveNeutralCamera(t *testing.T) {
	dir := t.TempDir()
	bar := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if err := SavePlayerState(dir, 12.5, 81.25, -72, 3, bar, 42); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, playerFile))
	if err != nil {
		t.Fatal(err)
	}
	var state InputState
	var c gameCamera
	w := &World{}
	if err := loadPlayerFile(dir, &state, &c, w); err != nil {
		t.Fatal(err)
	}
	if c.Position != newGameVec3(12.5, 81.25, -72) || w.seed != 42 || state.SelectedSlot != 3 || state.CurrentBlock != 4 {
		t.Fatal("player state did not round trip")
	}
	for size := 0; size < len(data); size++ {
		if err := os.WriteFile(filepath.Join(dir, playerFile), data[:size], 0600); err != nil {
			t.Fatal(err)
		}
		before := state
		beforeCamera := c
		if err := loadPlayerFile(dir, &state, &c, w); err == nil {
			t.Fatalf("accepted truncated player file of length %d", size)
		}
		if state != before || c != beforeCamera || w.seed != 42 {
			t.Fatalf("truncated file mutated state at length %d", size)
		}
	}
}
