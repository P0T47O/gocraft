package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Legacy drawing converts neutral layout rectangles only at its boundary.
type legacySurvivalLayout struct{ SurvivalLayout }

func legacySurvivalLayoutFor(w, h float32) legacySurvivalLayout {
	return legacySurvivalLayout{survivalLayout(w, h)}
}
func raylibRect(r uiRect) rl.Rectangle { return rl.NewRectangle(r.X, r.Y, r.Width, r.Height) }
func (l legacySurvivalLayout) Rect(x, y, w, h float32) rl.Rectangle {
	return raylibRect(l.SurvivalLayout.Rect(x, y, w, h))
}
func (l legacySurvivalLayout) Slot(i int) rl.Rectangle { return raylibRect(l.SurvivalLayout.Slot(i)) }
func (l legacySurvivalLayout) Row(i int) rl.Rectangle  { return raylibRect(l.SurvivalLayout.Row(i)) }
