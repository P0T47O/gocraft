package main

import "testing"

func TestRespawnRetriesUseWindowClock(t *testing.T) {
	preserveWindowInput(t)
	c := &Client{Outgoing: make(chan Packet, 8), done: make(chan struct{})}
	s := &InputState{RespawnWaiting: true}
	windowFrame.Time = 10
	s.updateRespawnRequest(c)
	if len(c.Outgoing) != 1 || s.RespawnLast != 10 {
		t.Fatal("initial respawn request missing")
	}
	if _, ok := (<-c.Outgoing).(*PacketRespawn); !ok {
		t.Fatal("unexpected packet")
	}
	windowFrame.Time = 10.5
	s.updateRespawnRequest(c)
	if len(c.Outgoing) != 0 {
		t.Fatal("respawn retry was not throttled")
	}
	windowFrame.Time = 11.1
	s.updateRespawnRequest(c)
	if len(c.Outgoing) != 1 || s.RespawnLast != 11.1 {
		t.Fatal("respawn retry did not use the native frame clock")
	}
	s.RespawnWaiting = false
	windowFrame.Time = 20
	s.updateRespawnRequest(c)
	if len(c.Outgoing) != 1 {
		t.Fatal("respawn retry continued after completion")
	}
	s.RespawnWaiting = true
	s.updateRespawnRequest(nil)
	if s.RespawnLast != 11.1 {
		t.Fatal("missing client advanced respawn timer")
	}
}
