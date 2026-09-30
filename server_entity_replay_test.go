package main

import (
	"fmt"
	"testing"
)

func TestSavedEntityReplayDoesNotSaturateLoginQueue(t *testing.T) {
	w := NewClientWorld()
	defer w.Close()
	w.entities = append(w.entities, &PlayerEntity{BaseEntity: BaseEntity{UUID: "tester", Type: EntityPlayer}})
	w.entities = append(w.entities, &MobEntity{BaseEntity: BaseEntity{UUID: "mob", Type: EntityPig}, Kind: "pig", Health: 10})
	for i := 0; i < 200; i++ {
		w.entities = append(w.entities, &ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("item-%d", i), Type: EntityItem}})
	}
	c := &ClientConnection{Name: "tester", Send: make(chan Packet, 128), done: make(chan struct{})}
	s := &Server{World: w}
	s.queueExistingEntities(c)
	if len(c.entityReplay) != 201 {
		t.Fatalf("queued %d saved entities, want 201 without self", len(c.entityReplay))
	}
	for i := 0; i < 80; i++ {
		c.Send <- &PacketInventoryUpdate{}
	}
	s.flushExistingEntities(c)
	if len(c.Send) != 80 || len(c.entityReplay) != 201 {
		t.Fatal("replay consumed reserved login/inventory queue space")
	}
	for len(c.Send) > 0 {
		<-c.Send
	}
	seen := make(map[string]bool)
	for tick := 0; len(c.entityReplay) > 0 && tick < 100; tick++ {
		s.flushExistingEntities(c)
		if len(c.Send) > entityReplayPacketsPerTick {
			t.Fatal("one streaming tick exceeded its entity packet budget")
		}
		for len(c.Send) > 0 {
			p := <-c.Send
			switch p := p.(type) {
			case *PacketEntitySpawn:
				if seen[p.EntityID] {
					t.Fatalf("duplicate entity spawn %q", p.EntityID)
				}
				seen[p.EntityID] = true
			case *PacketMobState:
				if !seen[p.IDString] {
					t.Fatal("mob state arrived before its spawn")
				}
			default:
				t.Fatalf("unexpected replay packet %T", p)
			}
		}
	}
	if len(c.entityReplay) != 0 || len(seen) != 201 {
		t.Fatalf("replay incomplete: pending=%d spawned=%d", len(c.entityReplay), len(seen))
	}
	select {
	case <-c.done:
		t.Fatal("entity replay disconnected the client")
	default:
	}
}

func TestSavedEntityReplaySkipsDespawnedEntity(t *testing.T) {
	w := NewClientWorld()
	defer w.Close()
	w.entities = []Entity{&ItemEntity{BaseEntity: BaseEntity{UUID: "gone", Type: EntityItem}}}
	c := &ClientConnection{Name: "tester", Send: make(chan Packet, 8), done: make(chan struct{})}
	s := &Server{World: w}
	s.queueExistingEntities(c)
	w.entities = nil
	s.flushExistingEntities(c)
	if len(c.Send) != 0 || len(c.entityReplay) != 0 {
		t.Fatal("despawned entity was replayed")
	}
}
