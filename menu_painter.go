package main

import "image/color"

// Menu layout and interaction are independent from GPU and window ownership.
type menuPainter interface {
	Size() (int, int)
	Rect(uiRect, color.RGBA)
	Text(string, float32, float32, float32, color.RGBA)
	Measure(string, float32) float32
	Clip(uiRect)
	Unclip()
}

var menuCanvas menuPainter

func menuWidth() int                  { w, _ := menuCanvas.Size(); return w }
func menuHeight() int                 { _, h := menuCanvas.Size(); return h }
func menuRect(r uiRect, c color.RGBA) { menuCanvas.Rect(r, c) }
func menuRectangle(x, y, w, h int32, c color.RGBA) {
	menuRect(newUIRect(float32(x), float32(y), float32(w), float32(h)), c)
}
func menuLines(r uiRect, thickness float32, c color.RGBA) {
	menuRect(newUIRect(r.X, r.Y, r.Width, thickness), c)
	menuRect(newUIRect(r.X, r.Y+r.Height-thickness, r.Width, thickness), c)
	menuRect(newUIRect(r.X, r.Y, thickness, r.Height), c)
	menuRect(newUIRect(r.X+r.Width-thickness, r.Y, thickness, r.Height), c)
}
func menuFade(c color.RGBA, a float32) color.RGBA {
	c.A = uint8(max(float32(0), min(float32(1), a)) * 255)
	return c
}
func menuMeasure(text string, size int32) int32 {
	return int32(menuCanvas.Measure(text, float32(size)))
}
func menuClip(x, y, w, h int32) {
	menuCanvas.Clip(newUIRect(float32(x), float32(y), float32(w), float32(h)))
}
func menuUnclip() { menuCanvas.Unclip() }
func menuGradient(x, y, w, h int32, top, bottom color.RGBA) {
	for i := int32(0); i < 64; i++ {
		t := float32(i) / 63
		mix := func(a, b uint8) uint8 { return uint8(float32(a)*(1-t) + float32(b)*t) }
		y0, y1 := y+h*i/64, y+h*(i+1)/64
		menuRectangle(x, y0, w, y1-y0, color.RGBA{mix(top.R, bottom.R), mix(top.G, bottom.G), mix(top.B, bottom.B), mix(top.A, bottom.A)})
	}
}
func (l SurvivalLayout) Text(txt string, x, y float32, size int32, c color.RGBA) {
	menuCanvas.Text(txt, l.X+x*l.S, l.Y+y*l.S, float32(size)*l.S, c)
}
func menuBox(r uiRect, fill, border color.RGBA) { menuRect(r, fill); menuLines(r, 1, border) }
func menuButton(r uiRect, label string, enabled, active bool, s float32) {
	fill, fg := invSlot, invMuted
	if enabled {
		fg = invText
	}
	if active {
		fill, fg = invAccent, invBackground
	}
	if enabled && uiContainsPoint(inputMousePosition(), r) {
		fill, fg = meshColor(83, 103, 78, 255), invText
		if active {
			fill, fg = meshColor(208, 233, 169, 255), invBackground
		}
	}
	menuBox(r, fill, invLine)
	fs := 14 * s
	menuCanvas.Text(label, r.X+(r.Width-menuCanvas.Measure(label, fs))/2, r.Y+(r.Height-fs)/2, fs, fg)
}
