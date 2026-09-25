package main

import (
	"bytes"
	"encoding/binary"
)

// PacketWorldTime is an authoritative clock snapshot. Periodic updates may be
// dropped under backpressure; login always sends a reliable initial snapshot.
type PacketWorldTime struct {
	Ticks int64
}

func (*PacketWorldTime) ID() int32 { return IDWorldTime }
func (p *PacketWorldTime) Encode(w *bytes.Buffer) error {
	return binary.Write(w, binary.LittleEndian, p.Ticks)
}
func (p *PacketWorldTime) Decode(r *bytes.Buffer) error {
	return binary.Read(r, binary.LittleEndian, &p.Ticks)
}
