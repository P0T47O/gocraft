package main

import (
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type testCursor struct {
	captured bool
	x, y     int
}

func (c *testCursor) SetCaptured(v bool)   { c.captured = v }
func (c *testCursor) SetPosition(x, y int) { c.x, c.y = x, y }

func preserveWindowInput(t *testing.T) {
	t.Helper()
	f, c := windowFrame, windowCursor
	t.Cleanup(func() { windowFrame, windowCursor = f, c })
}

func TestWindowInputSnapshot(t *testing.T) {
	preserveWindowInput(t)
	windowFrame = windowInputFrame{Width: 1280, Height: 720, Text: []rune("a中🙂"), Wheel: -1}
	windowFrame.Down[keyW], windowFrame.Pressed[keyW] = true, true
	windowFrame.MousePressed[mouseLeft] = true
	for i := 0; i < 2; i++ {
		if !inputKeyDown(keyW) || !inputKeyPressed(keyW) || !inputMousePressed(mouseLeft) || inputMouseWheel() != -1 {
			t.Fatal("queries consumed a frame edge")
		}
	}
	for _, want := range []rune{'a', '中', '🙂', 0, 0} {
		if got := inputChar(); got != want {
			t.Fatalf("character = %U, want %U", got, want)
		}
	}
	if inputKeyDown(-1) || inputKeyPressed(keyCount) || inputMouseDown(mouseButtonCount) || inputMousePressed(-1) {
		t.Fatal("invalid input index accepted")
	}
	windowFrame = windowInputFrame{}
	if inputKeyPressed(keyW) || inputMousePressed(mouseLeft) || inputChar() != 0 {
		t.Fatal("input leaked across frame reset")
	}
}

func TestSelectionOnlySendsChanges(t *testing.T) {
	preserveWindowInput(t)
	previousClient := client
	t.Cleanup(func() { client = previousClient })
	client = &Client{Outgoing: make(chan Packet, 8), done: make(chan struct{})}
	s := &InputState{SelectedSlot: 0}
	windowFrame = windowInputFrame{}
	s.UpdateSelection(true)
	if got := len(client.Outgoing); got != 0 {
		t.Fatalf("unchanged selection sent %d packets", got)
	}
	windowFrame.Wheel = -1
	s.UpdateSelection(true)
	if got := len(client.Outgoing); got != 1 || s.SelectedSlot != 1 {
		t.Fatalf("changed selection sent %d packets, slot %d", got, s.SelectedSlot)
	}
}

func TestCursorTransitionsDiscardWarpDelta(t *testing.T) {
	preserveWindowInput(t)
	c := &testCursor{}
	windowCursor = c
	windowFrame.Delta = uiPoint{30, 40}
	captureCursor()
	if !c.captured || windowFrame.Delta != (uiPoint{}) {
		t.Fatal("capture retained stale mouse movement")
	}
	positionCursor(640, 360)
	if c.x != 640 || c.y != 360 || inputMousePosition() != (uiPoint{640, 360}) {
		t.Fatal("cursor position was not synchronized")
	}
	releaseCursor()
	if c.captured {
		t.Fatal("cursor remained captured")
	}
}

func TestNeutralCameraConsumesMouseSnapshot(t *testing.T) {
	preserveWindowInput(t)
	mode := currentGameMode
	currentGameMode = ModeCreative
	t.Cleanup(func() { currentGameMode = mode })
	windowFrame = windowInputFrame{Delta: uiPoint{10, -20}}
	c := gameCamera{Position: newGameVec3(8, 80, 8)}
	s := &InputState{Sensitivity: .01}
	s.UpdateCamera(nil, &c)
	if math.Abs(float64(s.Yaw+.1)) > 1e-6 || math.Abs(float64(s.Pitch-.2)) > 1e-6 {
		t.Fatal("mouse snapshot was not applied")
	}
	if c.Position != newGameVec3(8, 80, 8) {
		t.Fatal("idle creative camera moved")
	}
	_, direction := s.RayFromCenter(c)
	if math.Abs(float64(gameVec3Length(direction)-1)) > 1e-6 {
		t.Fatal("aim direction is not normalized")
	}
}

func TestSharedGameplayHasNoWindowLibraryImports(t *testing.T) {
	// Keep the cleared boundary from silently growing dependencies again.
	for _, path := range []string{"main.go", "settings_window.go", "render_mesh_upload.go", "mesh_lighting_test.go", "webgpu_optimization_test.go", "webgpu_chunk_preview_test.go", "webgpu_region_preview_test.go", "webgpu_surface_preview_test.go", "webgpu_textured_preview_test.go", "webgpu_transparency_preview_test.go", "webgpu_preview_window_windows_test.go", "input.go", "game_camera.go", "game_math.go", "game_update.go", "game_packets.go", "game_session.go", "window_input.go", "inventory_input.go", "container_input.go", "inventory_layout.go", "ui.go", "types.go", "player_collision.go", "save.go", "save_player.go", "performance_monitor.go", "menu_screens.go", "ui_menu.go", "menu_painter.go", "menu_death.go", "ui_palette.go", "webgpu_game_windows.go", "native_game_windows.go", "window_win32_windows.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(name, "raylib") {
				t.Errorf("%s imports window library %s", path, name)
			}
		}
	}
}

func TestWebGPUAndPoseHaveNoRaylibImports(t *testing.T) {
	paths, err := filepath.Glob("webgpu*.go")
	if err != nil || len(paths) == 0 {
		t.Fatalf("find WebGPU files: %v", err)
	}
	paths = append(paths, "mob_pose.go", "mob_pose_test.go", "mob_test.go")
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(name, "raylib") {
				t.Errorf("%s imports %s", path, name)
			}
		}
	}
}
