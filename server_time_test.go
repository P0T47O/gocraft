package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestServerClockSyncAndPause(t *testing.T) {
	w := NewClientWorld()
	defer w.Close()
	s := &Server{World: w, Clients: make(map[string]*ClientConnection)}
	connection := &ClientConnection{Send: make(chan Packet, 2)}
	s.Clients["viewer"] = connection
	for i := 0; i < timeSyncInterval; i++ {
		s.tickWorldTime()
	}
	if got := int64(w.TimeTicks); got != initialWorldTime+timeSyncInterval {
		t.Fatalf("clock advanced to %d", got)
	}
	select {
	case pkt := <-connection.Send:
		if got := pkt.(*PacketWorldTime).Ticks; got != int64(w.TimeTicks) {
			t.Fatalf("synced %d instead of %d", got, int64(w.TimeTicks))
		}
	default:
		t.Fatal("server did not sync world time")
	}
	s.Paused.Store(true)
	s.Tick()
	if int64(w.TimeTicks) != initialWorldTime+timeSyncInterval {
		t.Fatal("paused server advanced world time")
	}
}

func TestWorldTimePersistsInLevel(t *testing.T) {
	path := t.TempDir()
	if err := SaveLevelState(path, 12345, 7*dayLengthTicks+18500); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, levelFile))
	if err != nil {
		t.Fatal(err)
	}
	var level LevelData
	if err := json.Unmarshal(data, &level); err != nil {
		t.Fatal(err)
	}
	if level.TimeTicks != 7*dayLengthTicks+18500 {
		t.Fatalf("saved clock %d", level.TimeTicks)
	}
	w := NewFlatWorld()
	defer w.Close()
	if _, _, _, _, err := LoadWorld(path, w); err != nil {
		t.Fatal(err)
	}
	if int64(w.TimeTicks) != level.TimeTicks {
		t.Fatalf("loaded clock %v", w.TimeTicks)
	}
}
