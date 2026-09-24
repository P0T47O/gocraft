package main

import (
	"testing"
	"time"
)

func TestDistantBuriedSection(t *testing.T) {
	far := float32(25 * chunkWidth * 25 * chunkWidth)
	near := float32(23 * chunkWidth * 23 * chunkWidth)
	if distantBuriedSection(80, 2, near) {
		t.Fatal("nearby cave detail should remain visible")
	}
	if !distantBuriedSection(80, 2, far) {
		t.Fatal("fully buried far section should be culled")
	}
	if distantBuriedSection(80, 3, far) {
		t.Fatal("surface safety section should remain visible")
	}
	if distantBuriedSection(0, 0, far) {
		t.Fatal("empty height map must not hide any sections")
	}
}

func TestChunkUnloadSchedule(t *testing.T) {
	var schedule chunkUnloadSchedule
	a, b := &World{}, &World{}
	now := time.Unix(100, 0)
	if !schedule.due(a, 0, 0, 36, now) || schedule.due(a, 0, 0, 36, now.Add(100*time.Millisecond)) {
		t.Fatal("first scan must run; unchanged next frame must not")
	}
	if !schedule.due(a, 1, 0, 36, now.Add(100*time.Millisecond)) || !schedule.due(a, 1, 0, 68, now.Add(200*time.Millisecond)) {
		t.Fatal("movement and radius change should trigger immediate scans")
	}
	if !schedule.due(b, 1, 0, 68, now.Add(300*time.Millisecond)) || !schedule.due(b, 1, 0, 68, now.Add(1300*time.Millisecond)) {
		t.Fatal("new session and elapsed interval should trigger scans")
	}
}
