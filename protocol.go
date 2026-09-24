package main

import (
	"bytes"
	"fmt"
	"io"
)

const protocolVersion = 7

// PacketID definitions
const (
	IDLogin           = 0x01
	IDChunkData       = 0x02
	IDBlockChange     = 0x03
	IDPlayerMove      = 0x04
	IDSpawnPoint      = 0x0A
	IDUnloadChunk     = 0x0B
	IDEntitySpawn     = 0x0C
	IDEntityDespawn   = 0x0D
	IDEntityMove      = 0x08
	IDPlayerAction    = 0x0E
	IDEntityMeta      = 0x0F
	IDGameMode        = 0x10
	IDInventoryUpdate = 0x11
	IDChat            = 0x12
	IDSlotChange      = 0x13
	IDClickWindow     = 0x14
	IDChunkRequest    = 0x15
	IDOpenWindow      = 0x16
	IDCraft           = 0x17
	IDBlockInteract   = 0x18
	IDVitals          = 0x19
	IDRespawn         = 0x1A
	IDChunkLight      = 0x1F
)

type Packet interface {
	ID() int32
	Encode(w *bytes.Buffer) error
	Decode(r *bytes.Buffer) error
}

func WritePacket(conn io.Writer, p Packet) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in WritePacket for ID %d: %v\n", p.ID(), r)
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	var buf bytes.Buffer
	// 1. Write Packet ID as VarInt
	WriteVarInt(&buf, p.ID())
	// 2. Write Payload
	if err := p.Encode(&buf); err != nil {
		return err
	}

	payload := buf.Bytes()
	// 3. Write Frame Length (Total: ID + Payload)
	var final bytes.Buffer
	WriteVarInt(&final, int32(len(payload)))
	final.Write(payload)

	_, err = conn.Write(final.Bytes())
	return err
}

const maxPacketSize = 4 * 1024 * 1024 // 4 MB limit per packet

func ReadPacket(conn io.Reader) (Packet, error) {
	// 1. Read Frame Length
	length, err := ReadVarIntFromReader(conn)
	if err != nil {
		return nil, err
	}
	if length <= 0 || length > maxPacketSize {
		return nil, fmt.Errorf("packet length out of range: %d", length)
	}

	// 2. Read full payload into buffer
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	r := bytes.NewBuffer(payload)

	// 3. Read Packet ID
	id, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}

	var p Packet
	switch id {
	case 0x1D:
		p = &PacketMobState{}
	case 0x1E:
		p = &PacketAttackMob{}
	case 0x1B:
		p = &PacketContainerState{}
	case 0x1C:
		p = &PacketContainerClick{}
	case IDVitals:
		p = &PacketVitals{}
	case IDRespawn:
		p = &PacketRespawn{}
	case IDLogin:
		p = &PacketLogin{}
	case IDChunkData:
		p = &PacketChunkData{}
	case IDChunkLight:
		p = &PacketChunkLight{}
	case IDBlockChange:
		p = &PacketBlockChange{}
	case IDPlayerMove:
		p = &PacketPlayerMove{}
	case IDSpawnPoint:
		p = &PacketSpawnPoint{}
	case IDUnloadChunk:
		p = &PacketUnloadChunk{}
	case IDEntitySpawn:
		p = &PacketEntitySpawn{}
	case IDEntityDespawn:
		p = &PacketEntityDespawn{}
	case IDEntityMove:
		p = &PacketEntityMove{}
	case IDPlayerAction:
		p = &PacketPlayerAction{}
	case IDEntityMeta:
		p = &PacketEntityMeta{}
	case IDGameMode:
		p = &PacketGameMode{}
	case IDInventoryUpdate:
		p = &PacketInventoryUpdate{}
	case IDSlotChange:
		p = &PacketSlotChange{}
	case IDClickWindow:
		p = &PacketClickWindow{}
	case IDOpenWindow:
		p = &PacketOpenWindow{}
	case IDCraft:
		p = &PacketCraft{}
	case IDBlockInteract:
		p = &PacketBlockInteract{}
	case IDChat:
		p = &PacketChat{}
	case IDChunkRequest:
		p = &PacketChunkRequest{}
	default:
		return nil, fmt.Errorf("unknown packet ID: %d", id)
	}

	if err := p.Decode(r); err != nil {
		return nil, err
	}
	return p, nil
}
