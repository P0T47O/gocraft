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
	hover := rl.CheckCollisionPointRec(rl.GetMousePosition(), rect)
	clicked := hover && rl.IsMouseButtonReleased(rl.MouseLeftButton)

	// If clicking button, clear focus from text fields
	if clicked {
		ui.ActiveID = ""
	}

	color := rl.DarkGray
	if active {
		if clicked {
			color = rl.Blue
		} else if hover {
			color = rl.Gray
		}
	} else {
		color = rl.Black
	}

	rl.DrawRectangleRec(rect, color)
	rl.DrawRectangleLinesEx(rect, 2, rl.LightGray)

	textSize := rl.MeasureTextEx(ui.Font, text, 20, 2)
	textPos := rl.Vector2{
		X: rect.X + (rect.Width-textSize.X)/2,
		Y: rect.Y + (rect.Height-textSize.Y)/2,
	}
	rl.DrawTextEx(ui.Font, text, textPos, 20, 2, rl.White)

	return active && clicked
}

func (ui *UIComponents) DrawTextField(rect rl.Rectangle, text *string, id string, maxLength int) {
	hover := rl.CheckCollisionPointRec(rl.GetMousePosition(), rect)
	if hover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		ui.ActiveID = id
	} else if rl.IsMouseButtonPressed(rl.MouseLeftButton) && !hover {
		if ui.ActiveID == id {
			ui.ActiveID = ""
		}
	}

	active := (ui.ActiveID == id)

	rl.DrawRectangleRec(rect, rl.DarkGray)
	if active || hover {
		rl.DrawRectangleLinesEx(rect, 2, rl.White)
	} else {
		rl.DrawRectangleLinesEx(rect, 2, rl.Gray)
	}

	// Simple Input Handling
	if active {
		key := rl.GetKeyPressed()
		for key != 0 {
			if key == int32(rl.KeyBackspace) {
				if len(*text) > 0 {
					*text = (*text)[:len(*text)-1]
				}
			} else if key >= 32 && key <= 126 && len(*text) < maxLength {
				*text += string(rune(key))
			}
			key = rl.GetKeyPressed()
		}
	}

	display := *text
	if active && (int(rl.GetTime()*2)%2 == 0) {
		display += "_"
	}

	rl.DrawTextEx(ui.Font, display, rl.Vector2{X: rect.X + 5, Y: rect.Y + (rect.Height-20)/2}, 20, 2, rl.White)
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
	rl.DrawRectangleRec(rect, rl.DarkGray)

	// Draw Fill
	t := (*value - min) / (max - min)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	fillRec := rl.NewRectangle(rect.X, rect.Y, rect.Width*float32(t), rect.Height)
	rl.DrawRectangleRec(fillRec, rl.Gray)

	rl.DrawRectangleLinesEx(rect, 2, rl.LightGray)

	// Draw Text Value
	valText := fmt.Sprintf("%.3f", *value)
	textSize := rl.MeasureTextEx(ui.Font, valText, 20, 2)
	textPos := rl.Vector2{
		X: rect.X + (rect.Width-textSize.X)/2,
		Y: rect.Y + (rect.Height-textSize.Y)/2,
	}
	rl.DrawTextEx(ui.Font, valText, textPos, 20, 2, rl.White)
}

func (ui *UIComponents) DrawDropdown(rect rl.Rectangle, options []string, selectedIndex *int, id string) bool {
	active := (ui.ActiveID == id)
	hover := rl.CheckCollisionPointRec(rl.GetMousePosition(), rect)

	// Toggle dropdown
	if hover && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
		if active {
			ui.ActiveID = ""
		} else {
			ui.ActiveID = id
		}
	}

	// Draw Header
	rl.DrawRectangleRec(rect, rl.DarkGray)
	rl.DrawRectangleLinesEx(rect, 2, rl.LightGray)

	currentText := ""
	if *selectedIndex >= 0 && *selectedIndex < len(options) {
		currentText = options[*selectedIndex]
	}

	ui.DrawLabel(rect.X+10, rect.Y+(rect.Height-20)/2, currentText, 20, rl.White)

	// Draw Arrow
	arrowX := rect.X + rect.Width - 30
	arrowY := rect.Y + rect.Height/2
	if active {
		rl.DrawTriangle(
			rl.NewVector2(arrowX, arrowY-5),
			rl.NewVector2(arrowX-5, arrowY+5),
			rl.NewVector2(arrowX+5, arrowY+5),
			rl.White,
		)
	} else {
		rl.DrawTriangle(
			rl.NewVector2(arrowX-5, arrowY-5),
			rl.NewVector2(arrowX, arrowY+5),
			rl.NewVector2(arrowX+5, arrowY-5),
			rl.White,
		)
	}

	// Draw Options if Active
	changed := false
	if active {
		itemHeight := rect.Height
		totalHeight := float32(len(options)) * itemHeight

		// Background for options
		// Draw on top of everything? In immediate mode without z-layering, we rely on draw order.
		// Main loop should draw active dropdown LAST.
		optsRect := rl.NewRectangle(rect.X, rect.Y+rect.Height, rect.Width, totalHeight)
		rl.DrawRectangleRec(optsRect, rl.DarkGray)
		rl.DrawRectangleLinesEx(optsRect, 2, rl.LightGray)

		for i, opt := range options {
			optRect := rl.NewRectangle(rect.X, rect.Y+rect.Height+float32(i)*itemHeight, rect.Width, itemHeight)
			optHover := rl.CheckCollisionPointRec(rl.GetMousePosition(), optRect)

			color := rl.DarkGray
			if optHover {
				color = rl.Gray
			}
			if i == *selectedIndex {
				color = rl.Blue
			}

			rl.DrawRectangleRec(optRect, color)
			ui.DrawLabel(optRect.X+10, optRect.Y+(optRect.Height-20)/2, opt, 20, rl.White)

			if optHover && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
				*selectedIndex = i
				ui.ActiveID = "" // Close dropdown
				changed = true
			}
		}

		// If clicked outside, close
		if rl.IsMouseButtonReleased(rl.MouseLeftButton) && !rl.CheckCollisionPointRec(rl.GetMousePosition(), optsRect) && !hover {
			ui.ActiveID = ""
		}
	}

	return changed
}
