package main

import (
	"fmt"
	"image/color"
)

// Input and presentation are separate from the native window implementation.
type UIComponents struct{ ActiveID, DraggingID string }

func NewUIComponents() *UIComponents { return &UIComponents{} }
func (ui *UIComponents) DrawButton(r uiRect, label string, enabled bool) bool {
	return ui.DrawAction(r, label, enabled, false)
}
func (ui *UIComponents) DrawAction(r uiRect, label string, enabled, primary bool) bool {
	menuButton(r, label, enabled, primary && enabled, r.Height/38)
	clicked := enabled && uiContainsPoint(inputMousePosition(), r) && inputMouseReleased(mouseLeft)
	if clicked {
		ui.ActiveID = ""
		ui.DraggingID = ""
	}
	return clicked
}
func (ui *UIComponents) DrawTextField(r uiRect, text *string, id string, maxLength int, blocked bool) {
	hover := uiContainsPoint(inputMousePosition(), r)
	if inputMousePressed(mouseLeft) {
		if !blocked && hover {
			ui.ActiveID = id
		} else if ui.ActiveID == id {
			ui.ActiveID = ""
		}
	}
	active := !blocked && ui.ActiveID == id
	menuRect(r, invBackground)
	border := invLine
	if active || hover {
		border = invText
	}
	menuLines(r, 2, border)
	if active {
		for c := inputChar(); c != 0; c = inputChar() {
			if c >= 32 && len([]rune(*text)) < maxLength {
				*text += string(c)
			}
		}
		if inputKeyPressed(keyBackspace) {
			chars := []rune(*text)
			if len(chars) > 0 {
				*text = string(chars[:len(chars)-1])
			}
		}
	}
	display := *text
	if active && int(inputTime()*2)%2 == 0 {
		display += "_"
	}
	fs := r.Height * .4
	for len(display) > 0 && menuCanvas.Measure(display, fs) > r.Width-20 {
		display = string([]rune(display)[1:])
	}
	menuCanvas.Text(display, r.X+10, r.Y+(r.Height-fs)/2, fs, invText)
}
func (ui *UIComponents) DrawLabel(x, y float32, text string, size float32, c color.RGBA) {
	menuCanvas.Text(text, x, y, size, c)
}
func (ui *UIComponents) DrawSlider(r uiRect, value *float32, low, high float32, id string) {
	if uiContainsPoint(inputMousePosition(), r) && inputMousePressed(mouseLeft) {
		ui.DraggingID = id
	}
	if inputMouseReleased(mouseLeft) {
		ui.DraggingID = ""
	}
	if ui.DraggingID == id && r.Width > 0 {
		*value = low + max(float32(0), min(float32(1), (inputMousePosition().X-r.X)/r.Width))*(high-low)
	}
	fraction := float32(0)
	if high > low {
		fraction = max(float32(0), min(float32(1), (*value-low)/(high-low)))
	}
	menuRect(r, invBackground)
	menuRect(newUIRect(r.X, r.Y, r.Width*fraction, r.Height), meshColor(70, 90, 63, 255))
	menuLines(r, 2, invLine)
	fs := r.Height * .48
	text := fmt.Sprintf("%.3f", *value)
	menuCanvas.Text(text, r.X+(r.Width-menuCanvas.Measure(text, fs))/2, r.Y+(r.Height-fs)/2, fs, invText)
	menuRect(newUIRect(r.X+fraction*(r.Width-4), r.Y, 4, r.Height), invAccent)
}
