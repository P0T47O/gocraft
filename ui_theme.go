package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Shared palette, text, panels and button states used throughout the game.
var (
	invBackground = rl.NewColor(30, 36, 38, 255)
	invPanel      = rl.NewColor(39, 47, 48, 255)
	invSlot       = rl.NewColor(48, 58, 58, 255)
	invLine       = rl.NewColor(69, 81, 78, 255)
	invText       = rl.NewColor(237, 237, 219, 255)
	invMuted      = rl.NewColor(155, 170, 159, 255)
	invAccent     = rl.NewColor(185, 216, 140, 255)
	invWarning    = rl.NewColor(225, 167, 132, 255)
)

func inventoryText(txt string, x, y float32, size int32, color rl.Color) {
	rl.DrawText(txt, int32(x), int32(y), size, color)
}
func (l SurvivalLayout) Text(txt string, x, y float32, size int32, color rl.Color) {
	inventoryText(txt, l.X+x*l.S, l.Y+y*l.S, int32(float32(size)*l.S), color)
}
func inventoryBox(r rl.Rectangle, fill, border rl.Color) {
	rl.DrawRectangleRec(r, fill)
	rl.DrawRectangleLinesEx(r, 1, border)
}
func inventoryButton(r rl.Rectangle, label string, enabled, active bool, s float32) {
	fill := invSlot
	fg := invMuted
	if enabled {
		fg = invText
	}
	if active {
		fill = invAccent
		fg = invBackground
	}
	if enabled && rl.CheckCollisionPointRec(rl.GetMousePosition(), r) {
		fill = rl.NewColor(83, 103, 78, 255)
		fg = invText
		if active {
			fill = rl.NewColor(208, 233, 169, 255)
			fg = invBackground
		}
	}
	inventoryBox(r, fill, invLine)
	fs := int32(14 * s)
	width := float32(rl.MeasureText(label, fs))
	inventoryText(label, r.X+(r.Width-width)/2, r.Y+(r.Height-float32(fs))/2, fs, fg)
}
