package main

import (
	"os"
	"testing"
	"time"
)

// Opt-in 32-radius cold-load measurement, no save files or GL context. Measures
// generation, lighting, publication and packet construction; NOT TCP or rendering.
func TestStreamingRadius32ColdLoad(t *testing.T) {
	if os.Getenv("GOCRAFT_STREAM_LOAD_TEST") != "1" {
		t.Skip("opt-in full-radius loading benchmark")
	}
	initBlockRegistry()
	oldMonitor := perfMon
	pm := new(PerformanceMonitor)
	perfMon = pm
	w := NewFlatWorld()
	w.seed = 12345
	defer func() { w.Close(); perfMon = oldMonitor }()
	c := &ClientConnection{Send: make(chan Packet, 128), KnownChunks: make(map[chunkKey]bool)}
	s := &Server{World: w, Clients: map[string]*ClientConnection{"load": c}, PendingChunks: make(map[chunkKey][]string)}
	w.StartBackend()
	start := time.Now()
	s.SendChunksAround("load", 0, 0, 32)
	total := len(s.PendingChunks)
	ticker := time.NewTicker(8 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	received := make(map[chunkKey]bool)
	for len(received) < total {
		select {
		case <-deadline.C:
			t.Fatalf("loading stalled: %d/%d, gen ready=%d pending=%d", len(received), total, len(w.genResults), len(s.PendingChunks))
		case <-ticker.C:
			s.processChunkStreaming()
			for len(c.Send) > 0 {
				if p, ok := (<-c.Send).(*PacketChunkData); ok {
					received[chunkKey{int(p.CX), int(p.CZ)}] = true
				}
			}
		}
	}
	elapsed := time.Since(start)
	t.Logf("server-only cold load: %d chunks in %.3fs (%.1f chunks/s)", total, elapsed.Seconds(), float64(total)/elapsed.Seconds())
	for i, name := range loadingPhaseNames {
		if n := pm.loading[i].calls.Load(); n > 0 {
			t.Logf("%s: calls=%d aggregate-work=%.1fms", name, n, float64(pm.loading[i].nanos.Load())/float64(time.Millisecond))
		}
	}
}
