package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVitalsPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_VITALS_PREVIEW") != "1" {
		t.Skip("opt-in graphical preview")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1280, 720, "Vitals preview")
	defer rl.CloseWindow()
	oldUI, oldMode := ui, currentGameMode
	defer func() { ui = oldUI; currentGameMode = oldMode }()
	ui = NewUIComponents()
	currentGameMode = ModeSurvival
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vitals-hud", "death-screen"} {
		target := rl.LoadRenderTexture(1280, 720)
		rl.BeginTextureMode(target)
		drawMenuBackdrop()
		s := &InputState{VitalsReady: true, Vitals: PacketVitals{Health: 13, Air: 150}, IsSwimming: true}
		if name == "death-screen" {
			s.Vitals.Health = 0
			s.Vitals.Cause = "Fell from a height"
			drawDeathScreen(s)
		} else {
			drawVitalsHUD(s)
		}
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(target.Texture)
		rl.ImageFlipVertical(img)
		path, _ := filepath.Abs(filepath.Join("work", name+".png"))
		ok := rl.ExportImage(*img, path)
		rl.UnloadImage(img)
		rl.UnloadRenderTexture(target)
		if !ok {
			t.Fatal("preview export failed")
		}
	}
}
