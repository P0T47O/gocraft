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
	c := &ClientConnection{Send: make(chan Packet, 8), StreamSend: make(chan Packet, 4)}
	for i := 0; i < cap(c.StreamSend); i++ {
		c.StreamSend <- &PacketChunkData{}
	}
	if chunkSendHasRoom(c) {
		t.Fatal("full bulk queue must stop chunk streaming")
	}
	<-c.StreamSend
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

func TestStreamingBackpressureLeavesGameplayQueueFree(t *testing.T) {
	c := &ClientConnection{Send: make(chan Packet, 128), StreamSend: make(chan Packet, serverStreamQueueCapacity), done: make(chan struct{})}
	for chunkSendHasRoom(c) {
		c.StreamSend <- &PacketChunkData{}
	}
	if len(c.Send) != 0 {
		t.Fatalf("bulk traffic leaked into gameplay queue: %d", len(c.Send))
	}
	for i := 0; i < 36; i++ {
		if !c.enqueue(&PacketInventoryUpdate{SlotID: int32(i)}) {
			t.Fatalf("inventory burst saturated gameplay queue at packet %d", i)
		}
	}

	// Several drops can be collected in one tick (notably a death pile or
	// clustered items in water). The final inventory state only needs one full
	// 36-slot snapshot, not one snapshot per collected entity.
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	chunk := lifecycleChunk(w, chunkKey{0, 0})
	chunk.blocks.Set(2, 70, 2, blockStone)
	player := &PlayerEntity{BaseEntity: BaseEntity{UUID: "collector", Type: EntityPlayer, X: 2, Y: 71.2, Z: 2}, GameMode: ModeSurvival}
	burst := &ClientConnection{Name: player.UUID, Send: make(chan Packet, 128), StreamSend: make(chan Packet, serverStreamQueueCapacity), KnownChunks: make(map[chunkKey]bool), done: make(chan struct{})}
	w.entities = []Entity{player}
	for i, id := range []string{"drop-a", "drop-b", "drop-c", "drop-d"} {
		w.entities = append(w.entities, &ItemEntity{
			BaseEntity: BaseEntity{UUID: id, Type: EntityItem, X: 2, Y: 71.2, Z: 2},
			ItemStack:  Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: int32(i)},
		})
	}
	srv := &Server{
		World:        w,
		Clients:      map[string]*ClientConnection{player.UUID: burst},
		LastSentPos:  make(map[string][3]float64),
		LastSentMeta: make(map[string]int32),
	}
	srv.UpdateEntities()
	inventoryUpdates := 0
	for len(burst.Send) > 0 {
		if (<-burst.Send).ID() == IDInventoryUpdate {
			inventoryUpdates++
		}
	}
	if inventoryUpdates != len(player.Inventory.Slots) {
		t.Fatalf("pickup burst sent %d inventory updates, want one %d-slot snapshot", inventoryUpdates, len(player.Inventory.Slots))
	}
	select {
	case <-burst.done:
		t.Fatal("pickup burst disconnected client")
	default:
	}
}

func TestStreamingFullPeerDoesNotBlockReadyPeer(t *testing.T) {
	initBlockRegistry()
	w := NewFlatWorld()
	defer w.Close()
	key := chunkKey{}
	c := w.ensureChunk(0, 0)
	c.generated = true
	full := &ClientConnection{Send: make(chan Packet, 8), StreamSend: make(chan Packet, 4), KnownChunks: map[chunkKey]bool{}}
	ready := &ClientConnection{Send: make(chan Packet, 8), StreamSend: make(chan Packet, 4), KnownChunks: map[chunkKey]bool{}}
	for i := 0; i < cap(full.StreamSend); i++ {
		full.StreamSend <- &PacketChunkData{}
	}
	s := &Server{World: w, Clients: map[string]*ClientConnection{"full": full, "ready": ready}, PendingChunks: map[chunkKey][]string{key: {"full", "ready"}}}
	s.processPendingChunks()
	if !ready.KnownChunks[key] || full.KnownChunks[key] || len(s.PendingChunks[key]) != 1 {
		t.Fatal("backpressure lost a request or blocked ready peer")
	}
	for len(full.StreamSend) > 0 {
		<-full.StreamSend
	}
	s.processPendingChunks()
	if !full.KnownChunks[key] || len(s.PendingChunks) != 0 {
		t.Fatal("slow peer failed to resume")
	}
	// Streaming must not mutate or publish while a single-player game is paused.
	s.Paused.Store(true)
	s.PendingChunks[key] = []string{"ready"}
	n := len(ready.StreamSend)
	s.processChunkStreaming()
	if len(ready.StreamSend) != n {
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
