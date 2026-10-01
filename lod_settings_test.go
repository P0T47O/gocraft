package main

import (
	"os"
	"testing"
)

func TestExperimentalLODSettingsOptIn(t *testing.T) {
	old := currentSettings
	defer func() { currentSettings = old }()
	t.Chdir(t.TempDir())
	currentSettings = nil
	if LoadSettings().ExperimentalLOD || horizonDistance() != 0 {
		t.Fatal("LOD must default to disabled")
	}
	if err := os.WriteFile(settingsFile, []byte(`{"RenderDistance":24,"HorizonDistance":96}`), 0600); err != nil {
		t.Fatal(err)
	}
	currentSettings = nil
	if LoadSettings().ExperimentalLOD || horizonDistance() != 0 {
		t.Fatal("old horizon settings must not opt in")
	}
	currentSettings.ExperimentalLOD = true
	SaveSettings()
	currentSettings = nil
	if !LoadSettings().ExperimentalLOD || horizonDistance() != 96 {
		t.Fatal("explicit opt-in did not persist")
	}
	currentSettings.ExperimentalLOD = false
	SaveSettings()
	currentSettings = nil
	if LoadSettings().ExperimentalLOD || horizonDistance() != 0 {
		t.Fatal("off state did not persist")
	}
}

func TestExperimentalLODToggleStopsAndRecaptures(t *testing.T) {
	old := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, HorizonDistance: 64}
	defer func() { currentSettings = old }()
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{})
	c.blocks.Set(8, 100, 8, blockStone)
	c.heightMap[8][8] = 101
	w.syncExperimentalLOD()
	w.recordChunkLODColumns(c, 0, 0)
	if w.lodCapture != nil || c.lodCaptured || len(w.lodObservedChunks) != 0 {
		t.Fatal("disabled LOD recorded chunks or started workers")
	}
	currentSettings.ExperimentalLOD = true
	w.syncExperimentalLOD()
	if w.lodCapture == nil || !c.lodCaptured {
		t.Fatal("enabling did not recapture existing chunk")
	}
	currentSettings.ExperimentalLOD = false
	w.syncExperimentalLOD()
	if w.lodCapture != nil || w.lodCache != nil || c.lodCaptured || len(w.lodObservedChunks) != 0 || len(w.lodColumns) != 0 {
		t.Fatal("disabling retained LOD runtime state")
	}
	currentSettings.ExperimentalLOD = true
	w.syncExperimentalLOD()
	if w.lodCapture == nil || !c.lodCaptured {
		t.Fatal("re-enable failed")
	}
}
