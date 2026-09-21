package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Window adaptation remains here until the native window/input owner migrates.
func ApplySettings() {
	if currentSettings == nil {
		return
	}
	if nativeWindowActive {
		if w, ok := windowCursor.(interface{ Resize(int, int) }); ok {
			w.Resize(currentSettings.ResolutionWidth, currentSettings.ResolutionHeight)
		}
		return
	}
	if rl.GetScreenWidth() != currentSettings.ResolutionWidth || rl.GetScreenHeight() != currentSettings.ResolutionHeight {
		rl.SetWindowSize(currentSettings.ResolutionWidth, currentSettings.ResolutionHeight)
	}
}
