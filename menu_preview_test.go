//go:build windows

package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// Opt-in actual renderer snapshots. No game saves/settings are read or changed.
func TestMenuPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_MENU_PREVIEW") != "1" {
		t.Skip("opt-in graphical preview")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
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
	for _, size := range [][2]int{{1280, 720}, {800, 600}} {
		capture := func(name string, draw func()) { captureNativeUI(t, r, name, size[0], size[1], nil, draw) }
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
