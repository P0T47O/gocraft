//go:build windows

package main

import (
	"errors"
	"fmt"
	"github.com/gogpu/wgpu"
	"testing"
)

func TestSurfaceRecoveryOnlyHandlesOutdated(t *testing.T) {
	for _, source := range []error{wgpu.ErrSurfaceOutdated, fmt.Errorf("present: %w", wgpu.ErrSurfaceOutdated)} {
		r := &webGPUWorldRenderer{}
		if err := r.recoverOutdatedSurface(source); err != nil || !r.surfaceNeedsReconfigure {
			t.Fatalf("recoverable surface error: %v", err)
		}
	}
	for _, source := range []error{nil, wgpu.ErrDeviceLost, wgpu.ErrSurfaceLost, wgpu.ErrOutOfMemory, wgpu.ErrTimeout, errors.New("hal: surface outdated")} {
		r := &webGPUWorldRenderer{}
		if err := r.recoverOutdatedSurface(source); err != source || r.surfaceNeedsReconfigure {
			t.Fatalf("unexpectedly suppressed %v", source)
		}
	}
}

func TestNativeResizeRequestWaitsForFrameBoundary(t *testing.T) {
	w := &nativeWindow{width: 1280, height: 720}
	w.Resize(1600, 900)
	w.Resize(1920, 1080)
	w.Resize(0, 0)
	if w.width != 1280 || w.height != 720 || w.pendingWidth != 1920 || w.pendingHeight != 1080 {
		t.Fatal("resize changed current frame dimensions or failed to coalesce")
	}
}
