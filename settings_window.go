package main

// ApplySettings delegates window changes to the active window owner without a graphics dependency.
// Native windows queue resizing until the next frame boundary.
func ApplySettings() {
	if currentSettings == nil {
		return
	}
	if w, ok := windowCursor.(interface{ Resize(int, int) }); ok {
		w.Resize(currentSettings.ResolutionWidth, currentSettings.ResolutionHeight)
	}
}
