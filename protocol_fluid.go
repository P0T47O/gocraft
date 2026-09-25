package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type FluidChange struct {
	X, Y, Z     int32
	Block, Meta byte
}

// Fluid changes share one critical packet per server tick, rather than one
// packet per changed voxel. The list is bounded by the simulation budget.
type PacketFluidDelta struct{ Changes []FluidChange }

func (*PacketFluidDelta) ID() int32 { return IDFluidDelta }
func (p *PacketFluidDelta) Encode(w *bytes.Buffer) error {
	if len(p.Changes) > 256 {
		return fmt.Errorf("too many fluid changes: %d", len(p.Changes))
	}
	if err := binary.Write(w, binary.BigEndian, uint16(len(p.Changes))); err != nil {
		return err
	}
	for _, c := range p.Changes {
		for _, v := range []int32{c.X, c.Y, c.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		w.WriteByte(c.Block)
		w.WriteByte(c.Meta)
	}
	return nil
}
func (p *PacketFluidDelta) Decode(r *bytes.Buffer) error {
	var count uint16
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return err
	}
	if count > 256 || r.Len() < int(count)*14 {
		return fmt.Errorf("invalid fluid change count: %d", count)
	}
	p.Changes = make([]FluidChange, count)
	for i := range p.Changes {
		c := &p.Changes[i]
		for _, v := range []*int32{&c.X, &c.Y, &c.Z} {
			if err := binary.Read(r, binary.BigEndian, v); err != nil {
				return err
			}
		}
		var err error
		if c.Block, err = r.ReadByte(); err != nil {
			return err
		}
		if c.Meta, err = r.ReadByte(); err != nil {
			return err
		}
	}
	return nil
}
