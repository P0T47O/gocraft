//go:build windows

package main

import (
	"os"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// Preview callers lock the OS thread and defer cleanup before unlocking it.
// A positive frame limit makes the same interactive tests hidden and finite.
type nativePreviewWindow struct {
	window        *nativeWindow
	frames, limit int
}

func newNativePreviewWindow(t *testing.T, width, height int) (*nativePreviewWindow, func()) {
	t.Helper()
	limit := 0
	if value := os.Getenv("GOCRAFT_WEBGPU_PREVIEW_FRAMES"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			t.Fatalf("invalid GOCRAFT_WEBGPU_PREVIEW_FRAMES: %q", value)
		}
		limit = n
	}
	oldFrame, oldCursor := windowFrame, windowCursor
	w, err := createNativeWindow(width, height, limit == 0)
	if err != nil {
		t.Fatal(err)
	}
	w.Poll()
	return &nativePreviewWindow{window: w, limit: limit}, func() {
		w.Close()
		windowFrame, windowCursor = oldFrame, oldCursor
	}
}

func (p *nativePreviewWindow) nextFrame() bool {
	if p.limit > 0 && p.frames >= p.limit {
		return false
	}
	// Automated previews exercise depth/surface recreation, not just first draw.
	if p.limit > 1 && p.frames == 1 {
		p.window.Resize(p.window.width+32, p.window.height+24)
	}
	for {
		p.window.Poll()
		if p.window.closed {
			return false
		}
		if !p.window.minimized && p.window.width > 0 && p.window.height > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.frames++
	return true
}

func TestNativePreviewFrameLimitAndCleanup(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("opt-in native window test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("GOCRAFT_WEBGPU_PREVIEW_FRAMES", "3")
	oldFrame, oldCursor := windowFrame, windowCursor
	p, closePreview := newNativePreviewWindow(t, 320, 240)
	defer closePreview()
	hwnd := p.window.hwnd
	w, h := p.window.width, p.window.height
	for i := 0; i < 3; i++ {
		if !p.nextFrame() {
			t.Fatalf("preview ended at frame %d", i)
		}
		if i == 1 && (p.window.width != w+32 || p.window.height != h+24) {
			t.Fatal("preview did not resize")
		}
	}
	if p.nextFrame() || p.frames != 3 {
		t.Fatal("preview exceeded frame budget")
	}
	closePreview()
	if _, exists := winWindows[hwnd]; exists {
		t.Fatal("preview window was not released")
	}
	if !reflect.DeepEqual(windowFrame, oldFrame) || windowCursor != oldCursor {
		t.Fatal("preview input state leaked")
	}
}
