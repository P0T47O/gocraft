package main

import rl "github.com/gen2brain/raylib-go/raylib"

type raylibCursorController struct{}

func (raylibCursorController) SetCaptured(captured bool) {
	if captured {
		rl.DisableCursor()
	} else {
		rl.EnableCursor()
	}
}
func (raylibCursorController) SetPosition(x, y int) { rl.SetMousePosition(x, y) }

// Temporary native input adapter, called after the owner's event poll.
func captureRaylibInput() {
	windowCursor = raylibCursorController{}
	keys := [...]int32{rl.KeyOne, rl.KeyTwo, rl.KeyThree, rl.KeyFour, rl.KeyFive, rl.KeySix, rl.KeySeven, rl.KeyEight, rl.KeyNine, rl.KeyA, rl.KeyD, rl.KeyE, rl.KeyQ, rl.KeyR, rl.KeyS, rl.KeyW, rl.KeySpace, rl.KeyEscape, rl.KeyEnter, rl.KeyBackspace, rl.KeyF1, rl.KeyF3, rl.KeyLeftShift, rl.KeyRightShift, rl.KeyLeftControl, rl.KeyRightControl}
	for k, code := range keys {
		windowFrame.Down[k] = rl.IsKeyDown(code)
		windowFrame.Pressed[k] = rl.IsKeyPressed(code)
	}
	for b, code := range [...]rl.MouseButton{rl.MouseLeftButton, rl.MouseRightButton} {
		windowFrame.MouseDown[b] = rl.IsMouseButtonDown(code)
		windowFrame.MousePressed[b] = rl.IsMouseButtonPressed(code)
		windowFrame.MouseReleased[b] = rl.IsMouseButtonReleased(code)
	}
	p, d := rl.GetMousePosition(), rl.GetMouseDelta()
	windowFrame.Mouse, windowFrame.Delta = uiPoint{p.X, p.Y}, uiPoint{d.X, d.Y}
	windowFrame.Wheel, windowFrame.Time = rl.GetMouseWheelMove(), rl.GetTime()
	windowFrame.Width, windowFrame.Height = rl.GetScreenWidth(), rl.GetScreenHeight()
	windowFrame.Text = windowFrame.Text[:0]
	windowFrame.textIndex = 0
	for r := rl.GetCharPressed(); r != 0; r = rl.GetCharPressed() {
		windowFrame.Text = append(windowFrame.Text, r)
	}
}

func (raylibCursorController) Resize(width, height int) {
	if int(rl.GetScreenWidth()) != width || int(rl.GetScreenHeight()) != height {
		rl.SetWindowSize(width, height)
	}
}
