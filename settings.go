package main

import (
	"encoding/json"
	"fmt"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// GameSettings holds persistent configuration
type GameSettings struct {
	Sensitivity      float32
	ResolutionWidth  int
	ResolutionHeight int
}

var currentSettings *GameSettings

const settingsFile = "settings.json"

// LoadSettings attempts to load settings.json, or returns defaults
func LoadSettings() *GameSettings {
	// Defaults
	settings := &GameSettings{
		Sensitivity:      0.005,
		ResolutionWidth:  1280,
		ResolutionHeight: 720,
	}

	data, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			fmt.Printf("Error parsing settings: %v\n", err)
		}
	} else {
		fmt.Println("No settings file found, using defaults.")
	}

	currentSettings = settings
	return settings
}

// SaveSettings writes current settings to disk
func SaveSettings() {
	if currentSettings == nil {
		return
	}
	data, err := json.MarshalIndent(currentSettings, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling settings: %v\n", err)
		return
	}

	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		fmt.Printf("Error saving settings: %v\n", err)
	}
}

// ApplySettings applies resolution and updates input state if active
func ApplySettings() {
	if currentSettings == nil {
		return
	}

	// Apply Resolution
	// Only apply if changed to avoid flicker?
	// Raylib SetWindowSize checks internally usually, but let's be safe
	if rl.GetScreenWidth() != currentSettings.ResolutionWidth || rl.GetScreenHeight() != currentSettings.ResolutionHeight {
		rl.SetWindowSize(currentSettings.ResolutionWidth, currentSettings.ResolutionHeight)
		// Recenter window if possible?
		// rl.SetWindowPosition(...) // Maybe later
	}

	// Sensitivity is read directly from currentSettings in input.go (if we link them)
	// Or we update the global InputState if it exists?
	// InputState is created per-session. So when NewInputState name is called,
	// it should read from currentSettings.
}
