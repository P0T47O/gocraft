package main

import (
	"bytes"
	"encoding/binary"
)

type PacketLogin struct {
	ProtocolVersion int32
	Username        string
	Seed            uint32
}

func (p *PacketLogin) ID() int32 { return IDLogin }
func (p *PacketLogin) Encode(w *bytes.Buffer) error {
	_ = WriteVarInt(w, p.ProtocolVersion)
	_ = WriteString(w, p.Username)
	return binary.Write(w, binary.LittleEndian, p.Seed)
}
func (p *PacketLogin) Decode(r *bytes.Buffer) error {
	var err error
	p.ProtocolVersion, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Username, err = ReadString(r)
	if err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &p.Seed)
}

type PacketPlayerMove struct {
	X, Y, Z    float64
	Yaw, Pitch float32
}

func (p *PacketPlayerMove) ID() int32 { return IDPlayerMove }
func (p *PacketPlayerMove) Encode(w *bytes.Buffer) error {
	binary.Write(w, binary.BigEndian, p.X)
	binary.Write(w, binary.BigEndian, p.Y)
	binary.Write(w, binary.BigEndian, p.Z)
	binary.Write(w, binary.BigEndian, p.Yaw)
	return binary.Write(w, binary.BigEndian, p.Pitch)
}
func (p *PacketPlayerMove) Decode(r *bytes.Buffer) error {
	if err := binary.Read(r, binary.BigEndian, &p.X); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.Y); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.Z); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.Yaw); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &p.Pitch)
}

type PacketSpawnPoint struct {
	X, Y, Z float64
}

func (p *PacketSpawnPoint) ID() int32 { return IDSpawnPoint }
func (p *PacketSpawnPoint) Encode(w *bytes.Buffer) error {
	binary.Write(w, binary.BigEndian, p.X)
	binary.Write(w, binary.BigEndian, p.Y)
	return binary.Write(w, binary.BigEndian, p.Z)
}
func (p *PacketSpawnPoint) Decode(r *bytes.Buffer) error {
	if err := binary.Read(r, binary.BigEndian, &p.X); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.Y); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &p.Z)
}
