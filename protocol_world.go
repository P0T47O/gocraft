package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type PacketChunkData struct {
	MetaData  []byte // Optional trailing field; older peers omit it.
	CX, CZ    int32
	Data      []byte
	LightData []byte
}

func (p *PacketChunkData) ID() int32 { return IDChunkData }
func (p *PacketChunkData) Encode(w *bytes.Buffer) error {
	_ = WriteVarInt(w, p.CX)
	_ = WriteVarInt(w, p.CZ)
	_ = WriteVarInt(w, int32(len(p.Data)))
	w.Write(p.Data)
	_ = WriteVarInt(w, int32(len(p.LightData)))
	w.Write(p.LightData)
	if p.MetaData != nil {
		_ = WriteVarInt(w, int32(len(p.MetaData)))
		w.Write(p.MetaData)
	}
	return nil
}
func (p *PacketChunkData) Decode(r *bytes.Buffer) error {
	var err error
	p.CX, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.CZ, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	len1, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if len1 != chunkWidth*chunkHeight*chunkWidth {
		return fmt.Errorf("invalid chunk block count: %d", len1)
	}
	p.Data = make([]byte, len1)
	if _, err := io.ReadFull(r, p.Data); err != nil {
		return err
	}
	len2, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if len2 != len1 {
		return fmt.Errorf("invalid chunk light count: %d", len2)
	}
	p.LightData = make([]byte, len2)
	if _, err := io.ReadFull(r, p.LightData); err != nil {
		return err
	}
	if r.Len() > 0 {
		count, err := ReadVarInt(r)
		if err != nil {
			return err
		}
		if count != len1 {
			return fmt.Errorf("invalid chunk metadata count: %d", count)
		}
		p.MetaData = make([]byte, count)
		if _, err := io.ReadFull(r, p.MetaData); err != nil {
			return err
		}
	}
	return nil
}

type PacketBlockChange struct {
	X, Y, Z int32
	BlockID byte
	Meta    byte
}

func (p *PacketBlockChange) ID() int32 { return IDBlockChange }
func (p *PacketBlockChange) Encode(w *bytes.Buffer) error {
	_ = WriteVarInt(w, p.X)
	_ = WriteVarInt(w, p.Y)
	_ = WriteVarInt(w, p.Z)
	w.WriteByte(p.BlockID)
	return w.WriteByte(p.Meta)
}
func (p *PacketBlockChange) Decode(r *bytes.Buffer) error {
	var err error
	p.X, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Y, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Z, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.BlockID); err != nil {
		return err
	}
	if r.Len() > 0 {
		p.Meta, err = r.ReadByte()
	}
	return err
}

type PacketChunkRequest struct {
	CX, CZ int32
}

func (p *PacketChunkRequest) ID() int32 { return IDChunkRequest }
func (p *PacketChunkRequest) Encode(w *bytes.Buffer) error {
	_ = WriteVarInt(w, p.CX)
	return WriteVarInt(w, p.CZ)
}
func (p *PacketChunkRequest) Decode(r *bytes.Buffer) error {
	var err error
	p.CX, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.CZ, err = ReadVarInt(r)
	return err
}

type PacketUnloadChunk struct {
	CX, CZ int32
}

func (p *PacketUnloadChunk) ID() int32 { return IDUnloadChunk }
func (p *PacketUnloadChunk) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.CX)
	return WriteVarInt(w, p.CZ)
}
func (p *PacketUnloadChunk) Decode(r *bytes.Buffer) error {
	var err error
	p.CX, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.CZ, err = ReadVarInt(r)
	return err
}
