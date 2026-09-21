//go:build windows

package main

import (
	"os"
	"testing"
)

func TestVitalsPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_VITALS_PREVIEW") != "1" {
		t.Skip("opt-in native preview")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	s := &InputState{VitalsReady: true, Vitals: PacketVitals{Health: 13, Air: 150}, IsSwimming: true}
	captureNativeUI(t, r, "vitals-hud", 1280, 720, s, nil)
	s.Vitals.Health = 0
	s.Vitals.Cause = "Fell from a height"
	captureNativeUI(t, r, "death-screen", 1280, 720, s, func() { drawDeathScreen(s) })
}
