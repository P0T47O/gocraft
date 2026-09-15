package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
)

// Full light snapshots for selected 16-high sections, ordered section/x/y/z.
// Geometry is untouched. TCP ordering ensures the initial full chunk arrives first.
type PacketChunkLight struct {
	CX, CZ   int32
	Sections uint16
	Data     []byte
}

func chunkLightSize(mask uint16) int {
	return bits.OnesCount16(mask) * chunkWidth * sectionHeight * chunkWidth
}
func (p *PacketChunkLight) ID() int32 { return IDChunkLight }
func (p *PacketChunkLight) Encode(w *bytes.Buffer) error {
	if p.Sections == 0 || len(p.Data) != chunkLightSize(p.Sections) {
		return fmt.Errorf("invalid chunk light payload")
	}
	WriteVarInt(w, p.CX)
	WriteVarInt(w, p.CZ)
	binary.Write(w, binary.LittleEndian, p.Sections)
	w.Write(p.Data)
	return nil
}
func (p *PacketChunkLight) Decode(r *bytes.Buffer) error {
	var err error
	if p.CX, err = ReadVarInt(r); err != nil {
		return err
	}
	if p.CZ, err = ReadVarInt(r); err != nil {
		return err
	}
	if err = binary.Read(r, binary.LittleEndian, &p.Sections); err != nil {
		return err
	}
	if p.Sections == 0 || r.Len() != chunkLightSize(p.Sections) {
		return fmt.Errorf("invalid chunk light payload length")
	}
	p.Data = make([]byte, r.Len())
	_, err = io.ReadFull(r, p.Data)
	return err
}
