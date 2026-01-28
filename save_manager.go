package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type SaveInfo struct {
	Path       string
	Name       string
	Seed       int64
	LastPlayed time.Time
	IsLegacy   bool
}

func ScanSaves() []SaveInfo {
	saves := []SaveInfo{}
	entries, err := os.ReadDir(RootSaveDir)
	if err != nil {
		// Ensure root exists
		os.MkdirAll(RootSaveDir, 0o755)
		return saves
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(RootSaveDir, entry.Name())
		info := SaveInfo{
			Path: path,
			Name: entry.Name(),
		}

		// Try to read level.json
		levelPath := filepath.Join(path, levelFile)
		data, err := os.ReadFile(levelPath)
		if err == nil {
			var ld LevelData
			if err := json.Unmarshal(data, &ld); err == nil {
				info.Name = ld.Name
				info.Seed = ld.Seed
				info.LastPlayed = time.Unix(ld.LastPlayed, 0)
			}
		} else {
			// Check if player.bin exists (Legacy save)
			if _, err := os.Stat(filepath.Join(path, playerFile)); err == nil {
				info.IsLegacy = true
				info.Name = entry.Name() + " (Legacy)"
				// We could try to read timestamp from file mod time
				if fi, err := entry.Info(); err == nil {
					info.LastPlayed = fi.ModTime()
				}
			} else {
				// Not a valid save folder
				continue
			}
		}
		saves = append(saves, info)
	}

	// Sort by last played (descending)
	sort.Slice(saves, func(i, j int) bool {
		return saves[i].LastPlayed.After(saves[j].LastPlayed)
	})

	return saves
}

func CreateNewSave(name string, seed int64) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name cannot be empty")
	}

	// Sanitize name for folder
	folderName := name // In real app, remove special chars
	path := filepath.Join(RootSaveDir, folderName)

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("save already exists")
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}

	// Create initial level.json
	ld := LevelData{
		Name:       name,
		Seed:       seed,
		LastPlayed: time.Now().Unix(),
		Version:    saveVersion,
	}

	bytes, err := json.MarshalIndent(ld, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(path, levelFile), bytes, 0o644)
	return path, err
}

func DeleteSave(path string) error {
	return os.RemoveAll(path)
}
