package main

import "testing"

func TestCreativeInventoryRowsAndIndices(t *testing.T) {
	l := inventoryLayoutFor(1280, 720)
	if got := l.creativeMaxScroll(0); got != 0 {
		t.Fatalf("empty catalog scroll = %d", got)
	}
	if got := l.creativeMaxScroll(l.Cols*l.Rows + l.Cols + 1); got != 2 {
		t.Fatalf("partial final row scroll = %d", got)
	}
	count := l.Cols*l.Rows + l.Cols + 1
	if got := l.creativeIndex(2, 0, 0, count); got != 2*l.Cols {
		t.Fatalf("first visible index = %d", got)
	}
	if got := l.creativeIndex(200, l.Rows-1, 0, count); got != count-1 {
		t.Fatalf("last visible index = %d", got)
	}
	if got := l.creativeIndex(-10, 0, 0, count); got != 0 {
		t.Fatalf("negative scroll index = %d", got)
	}
}

func TestCreativeInventoryWheelScroll(t *testing.T) {
	preserveWindowInput(t)
	oldMode, oldBlocks := currentGameMode, allBlocks
	t.Cleanup(func() { currentGameMode, allBlocks = oldMode, oldBlocks })
	allBlocks = make([]byte, 9*10)
	currentGameMode = ModeCreative
	windowFrame = windowInputFrame{Width: 1280, Height: 720, Wheel: -1}
	s := &InputState{}
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 1 {
		t.Fatalf("one wheel notch moved to row %d", s.InventoryScroll)
	}
	windowFrame.Wheel = -100
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 4 {
		t.Fatalf("scroll was not clamped to final row: %d", s.InventoryScroll)
	}
	windowFrame.Wheel = 100
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 0 {
		t.Fatalf("reverse scroll was not clamped: %d", s.InventoryScroll)
	}
	windowFrame.Wheel = -0.5
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 0 {
		t.Fatalf("fractional wheel moved too early: %d", s.InventoryScroll)
	}
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 1 {
		t.Fatalf("fractional wheel was lost: %d", s.InventoryScroll)
	}
	currentGameMode = ModeSurvival
	windowFrame.Wheel = -2
	s.UpdateInventoryScroll()
	if s.InventoryScroll != 1 {
		t.Fatal("survival wheel changed creative scroll")
	}
}
