package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// GameSettings holds persistent configuration
type GameSettings struct {
	Sensitivity      float32
	ResolutionWidth  int
	ResolutionHeight int
	PlayerName       string
	RenderDistance   int // Chunks; client-pull streaming supports live changes.
	Mipmaps          bool
	Anisotropy       int // 1 disables anisotropic filtering; otherwise 2/4/8/16.
}

const minRenderDistance = 8
const maxRenderDistance = 32
const defaultRenderDistance = 24

func renderDistance() int {
	if currentSettings == nil {
		return defaultRenderDistance
	}
	return clampRenderDistance(currentSettings.RenderDistance)
}

func clampRenderDistance(v int) int {
	if v == 0 {
		return defaultRenderDistance
	}
	return max(minRenderDistance, min(maxRenderDistance, v))
}

var currentSettings *GameSettings

const settingsFile = "settings.json"

// LoadSettings attempts to load settings.json, or returns defaults
func LoadSettings() *GameSettings {
	if currentSettings != nil {
		return currentSettings
	}

	// Defaults
	settings := &GameSettings{
		Sensitivity:      0.005,
		ResolutionWidth:  1280,
		ResolutionHeight: 720,
		PlayerName:       "Player",
		RenderDistance:   defaultRenderDistance,
		Mipmaps:          true,
		Anisotropy:       8,
	}

	data, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			fmt.Printf("Error parsing settings: %v\n", err)
		}
	} else {
		fmt.Println("No settings file found, using defaults.")
	}

	settings.RenderDistance = clampRenderDistance(settings.RenderDistance)
	settings.Anisotropy = normalizeAnisotropy(settings.Anisotropy)
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
