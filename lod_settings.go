package main

import "time"

// Run on the world owner. Disabling flushes existing observations, stops CPU
// workers and releases records; GPU resources are released by the next draw.
// Enabling recaptures loaded chunks with bounded admission, without a reload.
func (w *World) syncExperimentalLOD() {
	if !w.IsClient {
		return
	}
	if !experimentalLODEnabled() {
		if w.lodCapture != nil || w.lodCache != nil {
			w.finishLODObservedCapture()
			w.resetLODState()
		}
		w.lodEnablePending = nil
		return
	}
	if w.lodCapture == nil {
		w.startLODObservedCapture()
		w.openLODCache(".gocraft-cache/lod", w.lodCacheSource)
		for key, c := range w.chunks {
			if c.generated {
				w.lodEnablePending = append(w.lodEnablePending, key)
			}
		}
	}
	deadline := time.Now().Add(time.Millisecond)
	for n := 0; n < 2 && len(w.lodEnablePending) > 0 && time.Now().Before(deadline); n++ {
		if len(w.lodCapture.jobs) == cap(w.lodCapture.jobs) {
			return
		}
		key := w.lodEnablePending[len(w.lodEnablePending)-1]
		w.lodEnablePending = w.lodEnablePending[:len(w.lodEnablePending)-1]
		if c := w.getChunkIfGenerated(key.X, key.Z); c != nil && !c.lodCaptured {
			w.recordChunkLODColumns(c, key.X, key.Z)
		}
	}
}
