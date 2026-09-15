package main

import (
	"strings"
	"testing"
	"time"
)

func TestStreamingBudgetsAndBackpressure(t *testing.T) {
	for _, tc := range [][2]int{{0, 4}, {5, 5}, {32, 32}, {3209, 32}} {
		if streamingBatchSize(tc[0]) != tc[1] {
			t.Fatal(tc)
		}
	}
	c := &ClientConnection{Send: make(chan Packet, 8)}
	for i := 0; i < 6; i++ {
		c.Send <- &PacketChunkData{}
	}
	if chunkSendHasRoom(c) {
		t.Fatal("bulk traffic must reserve gameplay space")
	}
	<-c.Send
	if !chunkSendHasRoom(c) {
		t.Fatal("drained peer did not resume")
	}
	s := &Server{chunkOrder: []chunkPriority{{}}, chunkOrderAt: time.Now()}
	if s.chunkOrderNeedsRefresh(s.chunkOrderAt.Add(49 * time.Millisecond)) {
		t.Fatal("sort repeated too early")
	}
	if !s.chunkOrderNeedsRefresh(s.chunkOrderAt.Add(50 * time.Millisecond)) {
		t.Fatal("movement priority never refreshed")
	}
}

func TestStreamingFullPeerDoesNotBlockReadyPeer(t *testing.T) {
	initBlockRegistry()
	w := NewFlatWorld()
	defer w.Close()
	key := chunkKey{}
	c := w.ensureChunk(0, 0)
	c.generated = true
	full := &ClientConnection{Send: make(chan Packet, 8), KnownChunks: map[chunkKey]bool{}}
	ready := &ClientConnection{Send: make(chan Packet, 8), KnownChunks: map[chunkKey]bool{}}
	for i := 0; i < 6; i++ {
		full.Send <- &PacketChat{}
	}
	s := &Server{World: w, Clients: map[string]*ClientConnection{"full": full, "ready": ready}, PendingChunks: map[chunkKey][]string{key: {"full", "ready"}}}
	s.processPendingChunks()
	if !ready.KnownChunks[key] || full.KnownChunks[key] || len(s.PendingChunks[key]) != 1 {
		t.Fatal("backpressure lost a request or blocked ready peer")
	}
	for len(full.Send) > 0 {
		<-full.Send
	}
	s.processPendingChunks()
	if !full.KnownChunks[key] || len(s.PendingChunks) != 0 {
		t.Fatal("slow peer failed to resume")
	}
	// Streaming must not mutate or publish while a single-player game is paused.
	s.Paused.Store(true)
	s.PendingChunks[key] = []string{"ready"}
	n := len(ready.Send)
	s.processChunkStreaming()
	if len(ready.Send) != n {
		t.Fatal("paused streaming continued")
	}
}

func TestLoadingMetricsColumnsAndDrain(t *testing.T) {
	pm := new(PerformanceMonitor)
	pm.recordLoading(phaseGeneration, time.Now().Add(-time.Millisecond))
	if pm.loading[phaseGeneration].calls.Load() != 1 {
		t.Fatal("phase not recorded")
	}
	header, values := loadingCSVHeader(), pm.loadingCSVValues()
	if len(strings.Split(header, ",")) != len(strings.Split(values, ",")) {
		t.Fatal("CSV column mismatch")
	}
	if pm.loading[phaseGeneration].calls.Load() != 0 || pm.loading[phaseGeneration].nanos.Load() != 0 {
		t.Fatal("interval not drained")
	}
}
