package main

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestEntitySnapshotsLeaveRoomForLiveSpawn(t *testing.T) {
	c := &ClientConnection{Name: "tester", Send: make(chan Packet, 128), done: make(chan struct{})}
	s := &Server{Clients: map[string]*ClientConnection{c.Name: c}}
	// Reproduce the missed path: a whole-world movement/mob-state burst,
	// followed by live packet 12 and a complete inventory snapshot.
	for i := 0; i < 500; i++ {
		s.Broadcast(&PacketEntityMove{EntityID: fmt.Sprint(i)})
		s.Broadcast(&PacketMobState{IDString: fmt.Sprint(i)})
	}
	if len(c.Send) != 80 {
		t.Fatalf("snapshots occupied %d slots, want 80", len(c.Send))
	}
	s.Broadcast(&PacketEntitySpawn{EntityID: "new-drop", Type: EntityItem})
	for i := 0; i < 36; i++ {
		s.BroadcastTo(c.Name, &PacketInventoryUpdate{SlotID: int32(i)})
	}
	select {
	case <-c.done:
		t.Fatal("snapshot burst disconnected client on live spawn/inventory")
	default:
	}
	if len(c.outboundPending) != 0 || len(c.Send) != 117 {
		t.Fatal("reserved gameplay slots were not used")
	}
}

// net.Pipe makes the writer stall deterministically until the peer resumes
// reading, without relying on operating-system socket buffer sizes or sleeps.
type outboundWriteProbe struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (c *outboundWriteProbe) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.Conn.Write(p)
}

func TestOutboundReliableBurstSurvivesStalledSocketInOrder(t *testing.T) {
	serverConn, peer := net.Pipe()
	defer peer.Close()
	probe := &outboundWriteProbe{Conn: serverConn, started: make(chan struct{})}
	s := &Server{Clients: make(map[string]*ClientConnection), PacketCh: make(chan PacketWrapper, 1), Shutdown: make(chan bool)}
	finished := make(chan struct{})
	go func() { defer close(finished); s.handleNewConnection(probe) }()
	defer func() {
		peer.Close()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("connection writer failed to stop")
		}
	}()
	if err := WritePacket(peer, &PacketLogin{Username: "tester", ProtocolVersion: protocolVersion}); err != nil {
		t.Fatal(err)
	}
	var c *ClientConnection
	select {
	case wrap := <-s.PacketCh:
		c = wrap.Connection
	case <-time.After(5 * time.Second):
		t.Fatal("login did not reach world owner")
	}
	c.StreamSend <- &PacketChat{Message: "blocked bulk write"}
	select {
	case <-probe.started:
	case <-time.After(5 * time.Second):
		t.Fatal("socket writer did not start")
	}
	for i := 0; i < 500; i++ {
		s.Broadcast(&PacketEntityMove{EntityID: fmt.Sprint(i)})
	}
	snapshots := len(c.Send)
	const entities = 300
	for i := 0; i < entities; i++ {
		id := fmt.Sprint(i)
		s.Broadcast(&PacketEntitySpawn{EntityID: id, Type: EntityItem})
		s.Broadcast(&PacketEntityDespawn{EntityID: id})
	}
	s.BroadcastTo(c.Name, &PacketGameMode{Mode: ModeSurvival})
	// New replaceable snapshots must not pass queued reliable transitions.
	for i := 0; i < 500; i++ {
		s.Broadcast(&PacketEntityMove{EntityID: "newer"})
	}
	select {
	case <-c.done:
		t.Fatal("temporary stalled socket disconnected")
	default:
	}
	if len(c.outboundPending) == 0 {
		t.Fatal("fixture did not exercise reliable overflow")
	}
	peer.SetReadDeadline(time.Now().Add(5 * time.Second))
	read := func() Packet {
		p, err := ReadPacket(peer)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p, ok := read().(*PacketChat); !ok || p.Message != "blocked bulk write" {
		t.Fatal("lost in-flight bulk message")
	}
	for i := 0; i < snapshots; i++ {
		if read().ID() != IDEntityMove {
			t.Fatal("unexpected snapshot ordering")
		}
	}
	for i := 0; i < entities; i++ {
		spawn, ok := read().(*PacketEntitySpawn)
		if !ok || spawn.EntityID != fmt.Sprint(i) {
			t.Fatalf("spawn %d reordered or lost", i)
		}
		despawn, ok := read().(*PacketEntityDespawn)
		if !ok || despawn.EntityID != spawn.EntityID {
			t.Fatal("despawn preceded or lost its spawn")
		}
	}
	if read().ID() != IDGameMode {
		t.Fatal("new reliable packet overtook pending lifecycle packets")
	}
}

func TestOutboundBacklogBoundAndReplayPause(t *testing.T) {
	c := &ClientConnection{Send: make(chan Packet, 8), done: make(chan struct{})}
	for i := 0; i < 8+serverReliableOverflowCapacity; i++ {
		if !c.enqueue(&PacketEntitySpawn{EntityID: fmt.Sprint(i)}) {
			t.Fatal("overflow closed before its bound")
		}
	}
	if c.enqueueReplay([]Packet{&PacketEntitySpawn{}}) {
		t.Fatal("replay passed pending gameplay")
	}
	c.enqueueSnapshot(&PacketEntityMove{})
	if len(c.outboundPending) != serverReliableOverflowCapacity {
		t.Fatal("snapshot entered reliable backlog")
	}
	if c.enqueue(&PacketEntitySpawn{}) {
		t.Fatal("non-consuming peer grew beyond bounded backlog")
	}
	select {
	case <-c.done:
	default:
		t.Fatal("sustained full peer not closed")
	}
}
