package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"image/color"
)

type raylibMenuPainter struct{}

func init()                                           { menuCanvas = raylibMenuPainter{} }
func (raylibMenuPainter) Size() (int, int)            { return rl.GetScreenWidth(), rl.GetScreenHeight() }
func (raylibMenuPainter) Rect(r uiRect, c color.RGBA) { rl.DrawRectangleRec(raylibRect(r), c) }
func (raylibMenuPainter) Text(s string, x, y, size float32, c color.RGBA) {
	rl.DrawText(s, int32(x), int32(y), int32(size), c)
}
func (raylibMenuPainter) Measure(s string, size float32) float32 {
	return float32(rl.MeasureText(s, int32(size)))
}
func (raylibMenuPainter) Clip(r uiRect) {
	rl.BeginScissorMode(int32(r.X), int32(r.Y), int32(r.Width), int32(r.Height))
}
func (raylibMenuPainter) Unclip() { rl.EndScissorMode() }
