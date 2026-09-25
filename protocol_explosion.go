package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// One event carries both the visual cue and the authoritative terrain delta.
// A bounded list avoids flooding the critical queue with individual changes.
type PacketExplosion struct {
	X, Y, Z float64
	Radius  float32
	Removed []BlockPos
}

func (p *PacketExplosion) ID() int32 { return IDExplosion }

func (p *PacketExplosion) Encode(w *bytes.Buffer) error {
	if len(p.Removed) > 1024 {
		return fmt.Errorf("explosion too large: %d blocks", len(p.Removed))
	}
	for _, v := range []float64{p.X, p.Y, p.Z} {
		if err := binary.Write(w, binary.BigEndian, v); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.BigEndian, p.Radius); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint16(len(p.Removed))); err != nil {
		return err
	}
	for _, pos := range p.Removed {
		for _, v := range []int32{pos.X, pos.Y, pos.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *PacketExplosion) Decode(r *bytes.Buffer) error {
	for _, v := range []*float64{&p.X, &p.Y, &p.Z} {
		if err := binary.Read(r, binary.BigEndian, v); err != nil {
			return err
		}
	}
	if err := binary.Read(r, binary.BigEndian, &p.Radius); err != nil {
		return err
	}
	var count uint16
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return err
	}
	if count > 1024 || r.Len() < int(count)*12 {
		return fmt.Errorf("invalid explosion block count: %d", count)
	}
	p.Removed = make([]BlockPos, count)
	for i := range p.Removed {
		for _, v := range []*int32{&p.Removed[i].X, &p.Removed[i].Y, &p.Removed[i].Z} {
			if err := binary.Read(r, binary.BigEndian, v); err != nil {
				return err
			}
		}
	}
	return nil
}
