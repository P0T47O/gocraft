package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Opt-in actual renderer snapshots. No game saves/settings are read or changed.
func TestMenuPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_MENU_PREVIEW") != "1" {
		t.Skip("opt-in graphical preview")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1280, 720, "Menu preview")
	defer rl.CloseWindow()
	oldUI, oldSettings, oldPage, oldSaves, oldScroll := ui, currentSettings, menuPage, saveList, worldScroll
	oldPauseSettings, oldError, oldDelete := pauseSettings, menuError, pendingDelete
	defer func() {
		ui = oldUI
		currentSettings = oldSettings
		menuPage = oldPage
		saveList = oldSaves
		worldScroll = oldScroll
		pauseSettings = oldPauseSettings
		menuError = oldError
		pendingDelete = oldDelete
	}()
	ui = NewUIComponents()
	currentSettings = &GameSettings{PlayerName: "Builder", Sensitivity: 0.005, ResolutionWidth: 1280, ResolutionHeight: 720}
	saveList = nil
	for i := 0; i < 9; i++ {
		saveList = append(saveList, SaveInfo{Path: fmt.Sprintf("preview-world-%d", i), Name: fmt.Sprintf("Woodland settlement %02d", i+1), Seed: int64(12345 + i), LastPlayed: time.Date(2026, 9, 11, 18, 30, 0, 0, time.UTC)})
	}
	worldScroll = 0
	menuError = ""
	pendingDelete = ""
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	capture := func(name string, draw func()) {
		target := rl.LoadRenderTexture(int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()))
		rl.BeginTextureMode(target)
		rl.ClearBackground(invBackground)
		draw()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(target.Texture)
		rl.ImageFlipVertical(img)
		path, _ := filepath.Abs(filepath.Join("work", name+".png"))
		ok := rl.ExportImage(*img, path)
		rl.UnloadImage(img)
		rl.UnloadRenderTexture(target)
		if !ok {
			t.Fatal("capture failed", name)
		}
	}
	for _, size := range [][2]int{{1280, 720}, {800, 600}} {
		rl.SetWindowSize(size[0], size[1])
		for _, page := range []MenuPage{MenuMain, MenuSingleplayer, MenuCreateWorld, MenuMultiplayer, MenuSettings} {
			menuPage = page
			capture(fmt.Sprintf("menu-%d-%d", page, size[0]), drawMenu)
		}
		pauseSettings = false
		capture(fmt.Sprintf("pause-%d", size[0]), drawPauseMenu)
		pauseSettings = true
		capture(fmt.Sprintf("pause-settings-%d", size[0]), drawPauseMenu)
	}
}
