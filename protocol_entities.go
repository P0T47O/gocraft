package main

import (
	"bytes"
	"encoding/binary"
)

type PacketEntitySpawn struct {
	EntityID   string
	Type       EntityType
	X, Y, Z    float64
	Yaw, Pitch float32
	Metadata   int32
}

func (p *PacketEntitySpawn) ID() int32 { return IDEntitySpawn }
func (p *PacketEntitySpawn) Encode(w *bytes.Buffer) error {
	WriteString(w, p.EntityID)
	w.WriteByte(byte(p.Type))
	binary.Write(w, binary.BigEndian, p.X)
	binary.Write(w, binary.BigEndian, p.Y)
	binary.Write(w, binary.BigEndian, p.Z)
	binary.Write(w, binary.BigEndian, p.Yaw)
	binary.Write(w, binary.BigEndian, p.Pitch)
	return binary.Write(w, binary.BigEndian, p.Metadata)
}
func (p *PacketEntitySpawn) Decode(r *bytes.Buffer) error {
	var err error
	p.EntityID, err = ReadString(r)
	if err != nil {
		return err
	}
	t, err := r.ReadByte()
	if err != nil {
		return err
	}
	p.Type = EntityType(t)
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
	if err := binary.Read(r, binary.BigEndian, &p.Pitch); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &p.Metadata)
}

type PacketEntityDespawn struct {
	EntityID string
}

func (p *PacketEntityDespawn) ID() int32 { return IDEntityDespawn }
func (p *PacketEntityDespawn) Encode(w *bytes.Buffer) error {
	return WriteString(w, p.EntityID)
}
func (p *PacketEntityDespawn) Decode(r *bytes.Buffer) error {
	var err error
	p.EntityID, err = ReadString(r)
	return err
}

type PacketEntityMeta struct {
	EntityID string
	Metadata int32
}

func (p *PacketEntityMeta) ID() int32 { return IDEntityMeta }
func (p *PacketEntityMeta) Encode(w *bytes.Buffer) error {
	WriteString(w, p.EntityID)
	return binary.Write(w, binary.BigEndian, p.Metadata)
}
func (p *PacketEntityMeta) Decode(r *bytes.Buffer) error {
	var err error
	p.EntityID, err = ReadString(r)
	if err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &p.Metadata)
}

type PacketEntityMove struct {
	EntityID   string
	X, Y, Z    float64
	Yaw, Pitch float32
}

func (p *PacketEntityMove) ID() int32 { return IDEntityMove }
func (p *PacketEntityMove) Encode(w *bytes.Buffer) error {
	WriteString(w, p.EntityID)
	binary.Write(w, binary.BigEndian, p.X)
	binary.Write(w, binary.BigEndian, p.Y)
	binary.Write(w, binary.BigEndian, p.Z)
	binary.Write(w, binary.BigEndian, p.Yaw)
	return binary.Write(w, binary.BigEndian, p.Pitch)
}
func (p *PacketEntityMove) Decode(r *bytes.Buffer) error {
	var err error
	p.EntityID, err = ReadString(r)
	if err != nil {
		return err
	}
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
