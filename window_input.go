package main

// Application key identifiers, independent of a window library's key codes.
const (
	keyOne int32 = iota
	keyTwo
	keyThree
	keyFour
	keyFive
	keySix
	keySeven
	keyEight
	keyNine
	keyA
	keyD
	keyE
	keyQ
	keyR
	keyS
	keyW
	keySpace
	keyEscape
	keyEnter
	keyBackspace
	keyF1
	keyF3
	keyLeftShift
	keyRightShift
	keyLeftControl
	keyRightControl
	keyB
	keyCount
)

const (
	mouseLeft int32 = iota
	mouseRight
	mouseButtonCount
)

type uiPoint struct{ X, Y float32 }
type uiRect struct{ X, Y, Width, Height float32 }

func newUIRect(x, y, width, height float32) uiRect {
	return uiRect{x, y, width, height}
}

func uiContainsPoint(p uiPoint, r uiRect) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X <= r.X+r.Width && p.Y <= r.Y+r.Height
}

// Input is captured once by the window owner. Queries never consume key or
// button edges, so gameplay and UI see the same frame. Text is a single queue.
type windowInputFrame struct {
	Down, Pressed                          [keyCount]bool
	MouseDown, MousePressed, MouseReleased [mouseButtonCount]bool
	Mouse, Delta                           uiPoint
	Wheel                                  float32
	Width, Height                          int
	Time                                   float64
	Text                                   []rune
	textIndex                              int
}

type cursorController interface {
	SetCaptured(bool)
	SetPosition(int, int)
}

var windowFrame windowInputFrame
var windowCursor cursorController

func inputKeyDown(k int32) bool    { return k >= 0 && k < keyCount && windowFrame.Down[k] }
func inputKeyPressed(k int32) bool { return k >= 0 && k < keyCount && windowFrame.Pressed[k] }
func inputMouseDown(b int32) bool  { return b >= 0 && b < mouseButtonCount && windowFrame.MouseDown[b] }
func inputMousePressed(b int32) bool {
	return b >= 0 && b < mouseButtonCount && windowFrame.MousePressed[b]
}
func inputMousePosition() uiPoint { return windowFrame.Mouse }
func inputMouseDelta() uiPoint    { return windowFrame.Delta }
func inputMouseWheel() float32    { return windowFrame.Wheel }
func inputTime() float64          { return windowFrame.Time }
func windowWidth() int            { return windowFrame.Width }
func windowHeight() int           { return windowFrame.Height }
func inputChar() rune {
	if windowFrame.textIndex >= len(windowFrame.Text) {
		return 0
	}
	r := windowFrame.Text[windowFrame.textIndex]
	windowFrame.textIndex++
	return r
}
func captureCursor() {
	if windowCursor != nil {
		windowCursor.SetCaptured(true)
	}
	windowFrame.Delta = uiPoint{}
}
func releaseCursor() {
	if windowCursor != nil {
		windowCursor.SetCaptured(false)
	}
	windowFrame.Delta = uiPoint{}
}
func positionCursor(x, y int) {
	if windowCursor != nil {
		windowCursor.SetPosition(x, y)
	}
	windowFrame.Mouse = uiPoint{float32(x), float32(y)}
	windowFrame.Delta = uiPoint{}
}

func inputMouseReleased(b int32) bool {
	return b >= 0 && b < mouseButtonCount && windowFrame.MouseReleased[b]
}
