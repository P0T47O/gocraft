package main

import (
	"fmt"
	"testing"
	"time"
)

func setupStreamingTest(t *testing.T) {
	t.Helper()
	oldClient, oldWorld, oldPlan := client, world, chunkRequests
	oldPending, oldStarts, oldPM := pendingChunkRequests, chunkLoadStarts, perfMon
	client = &Client{Outgoing: make(chan Packet, 128), done: make(chan struct{})}
	world = NewClientWorld()
	pendingChunkRequests = make(map[chunkKey]time.Time)
	chunkLoadStarts = make(map[chunkKey]time.Time)
	chunkRequests = chunkRequestPlan{}
	perfMon = nil
	t.Cleanup(func() {
		world.Close()
		client, world, chunkRequests = oldClient, oldWorld, oldPlan
		pendingChunkRequests, chunkLoadStarts, perfMon = oldPending, oldStarts, oldPM
	})
}

func TestChunkStreamingRetryTeleportReset(t *testing.T) {
	setupStreamingTest(t)
	now := time.Unix(100, 0)
	pos := newGameVec3(0, 0, 0)
	requestMissingChunksAt(pos, now)
	key := chunkKey{}
	first := chunkLoadStarts[key]
	drain := func() {
		for len(client.Outgoing) > 0 {
			<-client.Outgoing
		}
	}
	drain()
	// Revisit the origin before timeout: it must not be sent again.
	chunkRequests.next = 0
	requestMissingChunksAt(pos, now.Add(2*time.Second))
	for len(client.Outgoing) > 0 {
		p := (<-client.Outgoing).(*PacketChunkRequest)
		if p.CX == 0 && p.CZ == 0 {
			t.Fatal("premature retry")
		}
	}
	chunkRequests.next = 0
	requestMissingChunksAt(pos, now.Add(3*time.Second))
	p := (<-client.Outgoing).(*PacketChunkRequest)
	if p.CX != 0 || p.CZ != 0 || chunkLoadStarts[key] != first || pendingChunkRequests[key] != now.Add(3*time.Second) {
		t.Fatal("retry timestamp or priority incorrect")
	}
	drain()
	requestMissingChunksAt(newGameVec3(160000, 0, -160000), now.Add(4*time.Second))
	if _, ok := chunkLoadStarts[key]; ok {
		t.Fatal("teleport retained old timer")
	}
	if _, ok := pendingChunkRequests[key]; ok {
		t.Fatal("teleport retained old request")
	}
	p = (<-client.Outgoing).(*PacketChunkRequest)
	if p.CX != 10000 || p.CZ != -10000 {
		t.Fatal("teleport did not prioritize destination")
	}
	resetChunkStreaming()
	if chunkRequests.valid || len(chunkLoadStarts) != 0 || len(pendingChunkRequests) != 0 {
		t.Fatal("session reset retained state")
	}
}

func TestChunkStreamingClosedConnection(t *testing.T) {
	setupStreamingTest(t)
	close(client.done)
	requestMissingChunksAt(gameVec3{}, time.Unix(100, 0))
	if len(client.Outgoing) != 0 || len(chunkLoadStarts) != 0 {
		t.Fatal("sent request after close")
	}
}

func TestChunkRequestPlanTranslation(t *testing.T) {
	for _, radius := range []int{16, 32, 64} {
		var moved, fresh chunkRequestPlan
		now := time.Unix(100, 0)
		moved.prepare(chunkKey{}, radius, now)
		storage := &moved.keys[0]
		destination := chunkKey{-35, 91}
		moved.prepare(destination, radius, now)
		fresh.prepare(destination, radius, now)
		if storage != &moved.keys[0] {
			t.Fatal("translation allocated new storage")
		}
		for i, k := range moved.keys {
			if k != fresh.keys[i] {
				t.Fatalf("radius %d: order changed at %d", radius, i)
			}
		}
		moved.prepare(destination, 8, now)
		for _, k := range moved.keys {
			if !moved.contains(k) {
				t.Fatal("view distance shrink retained outside key")
			}
		}
	}
}

func BenchmarkChunkRequestPlanMovement(b *testing.B) {
	for _, radius := range []int{16, 32, 64} {
		for _, rebuild := range []bool{false, true} {
			b.Run(fmt.Sprintf("radius%d/rebuild%t", radius, rebuild), func(b *testing.B) {
				var p chunkRequestPlan
				now := time.Unix(100, 0)
				p.prepare(chunkKey{}, radius, now)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if rebuild {
						p.valid = false
					}
					p.prepare(chunkKey{i + 1, 0}, radius, now)
				}
			})
		}
	}
}
