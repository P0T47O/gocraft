package main

import "testing"

type settingsWindowRecorder struct{ width, height, calls int }

func (*settingsWindowRecorder) SetCaptured(bool)     {}
func (*settingsWindowRecorder) SetPosition(int, int) {}
func (w *settingsWindowRecorder) Resize(width, height int) {
	w.width, w.height = width, height
	w.calls++
}

func TestSettingsDelegateToWindowOwner(t *testing.T) {
	oldSettings, oldCursor := currentSettings, windowCursor
	t.Cleanup(func() { currentSettings, windowCursor = oldSettings, oldCursor })
	w := &settingsWindowRecorder{}
	windowCursor = w
	currentSettings = nil
	ApplySettings()
	if w.calls != 0 {
		t.Fatal("nil settings changed the window")
	}
	currentSettings = &GameSettings{ResolutionWidth: 1600, ResolutionHeight: 900}
	ApplySettings()
	if w.calls != 1 || w.width != 1600 || w.height != 900 {
		t.Fatalf("unexpected resize: %+v", w)
	}
	windowCursor = nil
	ApplySettings() // Settings are also valid before window creation.
}
