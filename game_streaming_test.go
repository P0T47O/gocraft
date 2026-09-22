package main

import (
	"testing"
	"time"
)

func TestChunkRequestPlan(t *testing.T) {
	var p chunkRequestPlan
	now := time.Unix(100, 0)
	center := chunkKey{-10, 12}
	p.prepare(center, 32, now)
	if len(p.keys) != 3209 || p.keys[0] != center {
		t.Fatalf("bad disk: %d", len(p.keys))
	}
	seen := map[chunkKey]bool{}
	last := -1
	for _, k := range p.keys {
		x, z := k.X-center.X, k.Z-center.Z
		d := x*x + z*z
		if seen[k] || d < last || !p.contains(k) {
			t.Fatal("duplicate, out of radius, or unordered")
		}
		seen[k], last = true, d
	}
	p.next = 123
	p.prepare(center, 32, now)
	if p.next != 123 {
		t.Fatal("restarted traversal")
	}
	p.next = len(p.keys)
	p.prepare(center, 32, now.Add(time.Millisecond))
	if p.next != len(p.keys) {
		t.Fatal("busy rescan")
	}
	p.prepare(center, 32, now.Add(time.Second))
	if p.next != 0 {
		t.Fatal("no retry scan")
	}
	p.prepare(chunkKey{100, -100}, 16, now)
	if p.keys[0] != (chunkKey{100, -100}) || p.next != 0 {
		t.Fatal("teleport not reprioritized")
	}
}

func TestChunkStreamingBackpressure(t *testing.T) {
	oldClient, oldWorld, oldPlan := client, world, chunkRequests
	oldPending, oldStarts := pendingChunkRequests, chunkLoadStarts
	defer func() {
		client, world, chunkRequests = oldClient, oldWorld, oldPlan
		pendingChunkRequests, chunkLoadStarts = oldPending, oldStarts
	}()
	client = &Client{Outgoing: make(chan Packet, 4), done: make(chan struct{})}
	world = NewClientWorld()
	defer world.Close()
	pendingChunkRequests = make(map[chunkKey]time.Time)
	chunkLoadStarts = make(map[chunkKey]time.Time)
	chunkRequests = chunkRequestPlan{}
	requestMissingChunks(newGameVec3(-1, 0, -1))
	if len(client.Outgoing) != 2 || len(pendingChunkRequests) != 2 {
		t.Fatal("did not reserve gameplay capacity")
	}
	cursor := chunkRequests.next
	requestMissingChunks(newGameVec3(-1, 0, -1))
	if cursor != chunkRequests.next {
		t.Fatal("dropped deferred request")
	}
	select {
	case <-client.done:
		t.Fatal("backpressure disconnected client")
	default:
	}
	first := (<-client.Outgoing).(*PacketChunkRequest)
	if first.CX != -1 || first.CZ != -1 {
		t.Fatal("negative coordinate rounding")
	}
	<-client.Outgoing
	requestMissingChunks(newGameVec3(-1, 0, -1))
	if len(pendingChunkRequests) != 4 {
		t.Fatal("did not resume")
	}
}

func TestChunkReadyLatency(t *testing.T) {
	oldPM, oldStarts := perfMon, chunkLoadStarts
	defer func() { perfMon, chunkLoadStarts = oldPM, oldStarts }()
	perfMon = &PerformanceMonitor{}
	key := chunkKey{1, 2}
	chunkLoadStarts = map[chunkKey]time.Time{key: time.Now().Add(-time.Second)}
	c := &Chunk{sectionDirty: make([]bool, sectionCount)}
	c.sectionBlocks[0], c.sectionDirty[0] = 1, true
	recordChunkReady(key, c)
	if len(perfMon.readyLatency) != 0 {
		t.Fatal("recorded unfinished mesh")
	}
	c.sectionDirty[0] = false
	recordChunkReady(key, c)
	recordChunkReady(key, c)
	if len(perfMon.readyLatency) != 1 || perfMon.readyLatency[0] < 1000 || len(chunkLoadStarts) != 0 {
		t.Fatal("bad completion latency")
	}
}

func BenchmarkChunkRequestPlanStationary(b *testing.B) {
	var p chunkRequestPlan
	now := time.Now()
	p.prepare(chunkKey{}, 32, now)
	p.next = len(p.keys)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.prepare(chunkKey{}, 32, now)
	}
}
