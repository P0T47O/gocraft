//go:build windows

package main

import (
	"errors"
	"github.com/gogpu/wgpu"
)

// Called once per draw, before acquiring a surface texture. A resize can make
// the swapchain stale even if the following frame has the same dimensions.
func (r *webGPUWorldRenderer) prepareSurface(width, height uint32) error {
	if !r.surfaceNeedsReconfigure && width == r.width && height == r.height {
		return nil
	}
	r.width, r.height = width, height
	r.surfaceNeedsReconfigure = true
	if err := r.configureSurface(); err != nil {
		return err
	}
	r.surfaceNeedsReconfigure = false
	return nil
}

// Run after frame-local views/commands are released. Skip this frame and
// rebuild on the next one: never replay menu interactions or spin in a retry
// loop while Windows is still resizing. Device/surface loss remains fatal.
func (r *webGPUWorldRenderer) recoverOutdatedSurface(err error) error {
	if !errors.Is(err, wgpu.ErrSurfaceOutdated) {
		return err
	}
	if r.surface != nil {
		r.surface.DiscardTexture()
	}
	r.surfaceNeedsReconfigure = true
	return nil
}
