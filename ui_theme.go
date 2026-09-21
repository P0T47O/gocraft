package main

import rl "github.com/gen2brain/raylib-go/raylib"

func inventoryText(txt string, x, y float32, size int32, color rl.Color) {
	rl.DrawText(txt, int32(x), int32(y), size, color)
}
func (l legacySurvivalLayout) Text(txt string, x, y float32, size int32, color rl.Color) {
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
