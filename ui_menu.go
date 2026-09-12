package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Simple IMGUI-style UI components
type UIComponents struct {
	Font       rl.Font
	ActiveID   string // ID of the currently active/focused element
	DraggingID string // ID of slider currently being dragged
}

func NewUIComponents() *UIComponents {
	return &UIComponents{
		Font: rl.GetFontDefault(),
	}
}

func (ui *UIComponents) DrawButton(rect rl.Rectangle, text string, active bool) bool {
	return ui.DrawAction(rect, text, active, false)
}

func (ui *UIComponents) DrawAction(rect rl.Rectangle, text string, enabled, primary bool) bool {
	inventoryButton(rect, text, enabled, primary && enabled, rect.Height/38)
	clicked := enabled && rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) && rl.IsMouseButtonReleased(rl.MouseLeftButton)
	if clicked {
		ui.ActiveID = ""
		ui.DraggingID = ""
	}
	return clicked
}

func (ui *UIComponents) DrawTextField(rect rl.Rectangle, text *string, id string, maxLength int, blocked bool) {
	hover := rl.CheckCollisionPointRec(rl.GetMousePosition(), rect)
	if !blocked && hover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		ui.ActiveID = id
	} else if rl.IsMouseButtonPressed(rl.MouseLeftButton) && !hover {
		if ui.ActiveID == id {
			ui.ActiveID = ""
		}
	}

	active := !blocked && (ui.ActiveID == id)

	rl.DrawRectangleRec(rect, invBackground)
	if active || hover {
		rl.DrawRectangleLinesEx(rect, 2, invText)
	} else {
		rl.DrawRectangleLinesEx(rect, 2, invLine)
	}

	// Simple Input Handling
	if active {
		// Handle Character Input
		char := rl.GetCharPressed()
		for char != 0 {
			if char >= 32 && len([]rune(*text)) < maxLength {
				*text += string(char)
			}
			char = rl.GetCharPressed()
		}

		// Handle Special Keys
		if rl.IsKeyPressed(rl.KeyBackspace) {
			if len(*text) > 0 {
				// Handle UTF-8 backspace properly-ish (assuming simple runes for now)
				runes := []rune(*text)
				if len(runes) > 0 {
					*text = string(runes[:len(runes)-1])
				}
			}
		}
		// Repeat Backspace if held
		if rl.IsKeyDown(rl.KeyBackspace) {
			// Simple counter-based repeat could go here, but for now single press is safer/simpler
			// forcing repeated tapping or holding logic requires state.
			// Let's stick to IsKeyPressed for single delete to avoid accidental wipe.
		}
	}

	display := *text
	if active && (int(rl.GetTime()*2)%2 == 0) {
		display += "_"
	}

	fs := rect.Height * 0.4
	for len(display) > 0 && rl.MeasureTextEx(ui.Font, display, fs, 1).X > rect.Width-20 {
		display = string([]rune(display)[1:])
	}
	rl.DrawTextEx(ui.Font, display, rl.NewVector2(rect.X+10, rect.Y+(rect.Height-fs)/2), fs, 1, invText)
}

func (ui *UIComponents) DrawLabel(x, y float32, text string, fontSize float32, color rl.Color) {
	rl.DrawTextEx(ui.Font, text, rl.Vector2{X: x, Y: y}, fontSize, 2, color)
}

func (ui *UIComponents) DrawSlider(rect rl.Rectangle, value *float32, min, max float32, id string) {
	hover := rl.CheckCollisionPointRec(rl.GetMousePosition(), rect)

	if hover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		ui.DraggingID = id
	}
	if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
		ui.DraggingID = ""
	}

	if ui.DraggingID == id {
		mouseBefore := rl.GetMousePosition().X
		t := (mouseBefore - rect.X) / rect.Width
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		*value = min + float32(t)*(max-min)
	}

	// Draw Background
	rl.DrawRectangleRec(rect, invBackground)

	// Draw Fill
	t := (*value - min) / (max - min)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	fillRec := rl.NewRectangle(rect.X, rect.Y, rect.Width*float32(t), rect.Height)
	rl.DrawRectangleRec(fillRec, rl.NewColor(70, 90, 63, 255))

	rl.DrawRectangleLinesEx(rect, 2, invLine)

	// Value and handle scale with the component rather than screen pixels.
	fs := rect.Height * 0.48
	valText := fmt.Sprintf("%.3f", *value)
	textSize := rl.MeasureTextEx(ui.Font, valText, fs, 1)
	rl.DrawTextEx(ui.Font, valText, rl.NewVector2(rect.X+(rect.Width-textSize.X)/2, rect.Y+(rect.Height-fs)/2), fs, 1, invText)
	handle := rl.NewRectangle(rect.X+t*(rect.Width-4), rect.Y, 4, rect.Height)
	rl.DrawRectangleRec(handle, invAccent)
}
