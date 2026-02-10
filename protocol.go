package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

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
)

type PacketClickWindow struct {
	SlotID     int32
	Button     int32 // 0: Left, 1: Right
	IsCreative bool  // Whether source is creative palette (for server logic)
	// Mode? 0: Click, 1: ShiftClick, 2: Drop, etc. For now just Button.
}

// ... existing PacketClickWindow methods ...
func (p *PacketClickWindow) ID() int32 { return IDClickWindow }
func (p *PacketClickWindow) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.SlotID)
	WriteVarInt(w, p.Button)
	if p.IsCreative {
		w.WriteByte(1)
	} else {
		w.WriteByte(0)
	}
	return nil
}
func (p *PacketClickWindow) Decode(r *bytes.Buffer) error {
	var err error
	p.SlotID, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Button, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	b, _ := r.ReadByte()
	p.IsCreative = (b == 1)
	return nil
}

type PacketBlockInteract struct {
	X, Y, Z int32
	Action  int32 // 0: Interact
}

func (p *PacketBlockInteract) ID() int32 { return IDBlockInteract }
func (p *PacketBlockInteract) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.X)
	WriteVarInt(w, p.Y)
	WriteVarInt(w, p.Z)
	return WriteVarInt(w, p.Action)
}
func (p *PacketBlockInteract) Decode(r *bytes.Buffer) error {
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
	p.Action, err = ReadVarInt(r)
	return err
}

type PacketOpenWindow struct {
	WindowID   byte
	WindowType byte // 0: Inventory, 1: Chest, 2: Workbench
}

func (p *PacketOpenWindow) ID() int32 { return IDOpenWindow }
func (p *PacketOpenWindow) Encode(w *bytes.Buffer) error {
	w.WriteByte(p.WindowID)
	w.WriteByte(p.WindowType)
	return nil
}
func (p *PacketOpenWindow) Decode(r *bytes.Buffer) error {
	var err error
	p.WindowID, err = r.ReadByte()
	if err != nil {
		return err
	}
	p.WindowType, err = r.ReadByte()
	return err
}

type PacketCraft struct {
	RecipeID int32
}

func (p *PacketCraft) ID() int32 { return IDCraft }
func (p *PacketCraft) Encode(w *bytes.Buffer) error {
	return WriteVarInt(w, p.RecipeID)
}
func (p *PacketCraft) Decode(r *bytes.Buffer) error {
	var err error
	p.RecipeID, err = ReadVarInt(r)
	return err
}

type PacketSlotChange struct {
	Slot int32
}

func (p *PacketSlotChange) ID() int32 { return IDSlotChange }
func (p *PacketSlotChange) Encode(w *bytes.Buffer) error {
	return WriteVarInt(w, p.Slot)
}
func (p *PacketSlotChange) Decode(r *bytes.Buffer) error {
	var err error
	p.Slot, err = ReadVarInt(r)
	return err
}

type PacketChat struct {
	Message string
}

func (p *PacketChat) ID() int32 { return IDChat }
func (p *PacketChat) Encode(w *bytes.Buffer) error {
	return WriteString(w, p.Message)
}
func (p *PacketChat) Decode(r *bytes.Buffer) error {
	var err error
	p.Message, err = ReadString(r)
	return err
}

type PacketGameMode struct {
	Mode byte // 0: Creative, 1: Survival
}

func (p *PacketGameMode) ID() int32 { return IDGameMode }
func (p *PacketGameMode) Encode(w *bytes.Buffer) error {
	return binary.Write(w, binary.LittleEndian, p.Mode)
}
func (p *PacketGameMode) Decode(r *bytes.Buffer) error {
	return binary.Read(r, binary.LittleEndian, &p.Mode)
}

type PacketInventoryUpdate struct {
	SlotID int32 // 0-8: Hotbar, 9-35: Inventory
	ItemID int32
	Count  int32
}

func (p *PacketInventoryUpdate) ID() int32 { return IDInventoryUpdate }
func (p *PacketInventoryUpdate) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.SlotID)
	WriteVarInt(w, p.ItemID)
	WriteVarInt(w, p.Count)
	return nil
}
func (p *PacketInventoryUpdate) Decode(r *bytes.Buffer) error {
	var err error
	if p.SlotID, err = ReadVarInt(r); err != nil {
		return err
	}
	if p.ItemID, err = ReadVarInt(r); err != nil {
		return err
	}
	if p.Count, err = ReadVarInt(r); err != nil {
		return err
	}
	return nil
}

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

func ReadPacket(conn io.Reader) (Packet, error) {
	// 1. Read Frame Length
	length, err := ReadVarIntFromReader(conn)
	if err != nil {
		return nil, err
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
	case IDLogin:
		p = &PacketLogin{}
	case IDChunkData:
		p = &PacketChunkData{}
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

type PacketChunkData struct {
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
	len1, _ := ReadVarInt(r)
	p.Data = make([]byte, len1)
	r.Read(p.Data)
	len2, _ := ReadVarInt(r)
	p.LightData = make([]byte, len2)
	r.Read(p.LightData)
	return nil
}

type PacketBlockChange struct {
	X, Y, Z int32
	BlockID byte
}

func (p *PacketBlockChange) ID() int32 { return IDBlockChange }
func (p *PacketBlockChange) Encode(w *bytes.Buffer) error {
	_ = WriteVarInt(w, p.X)
	_ = WriteVarInt(w, p.Y)
	_ = WriteVarInt(w, p.Z)
	return binary.Write(w, binary.BigEndian, p.BlockID)
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
	return binary.Read(r, binary.BigEndian, &p.BlockID)
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
	binary.Read(r, binary.BigEndian, &p.X)
	binary.Read(r, binary.BigEndian, &p.Y)
	binary.Read(r, binary.BigEndian, &p.Z)
	binary.Read(r, binary.BigEndian, &p.Yaw)
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
	binary.Read(r, binary.BigEndian, &p.X)
	binary.Read(r, binary.BigEndian, &p.Y)
	return binary.Read(r, binary.BigEndian, &p.Z)
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
	t, _ := r.ReadByte()
	p.Type = EntityType(t)
	binary.Read(r, binary.BigEndian, &p.X)
	binary.Read(r, binary.BigEndian, &p.Y)
	binary.Read(r, binary.BigEndian, &p.Z)
	binary.Read(r, binary.BigEndian, &p.Yaw)
	binary.Read(r, binary.BigEndian, &p.Pitch)
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
	binary.Read(r, binary.BigEndian, &p.X)
	binary.Read(r, binary.BigEndian, &p.Y)
	binary.Read(r, binary.BigEndian, &p.Z)
	binary.Read(r, binary.BigEndian, &p.Yaw)
	return binary.Read(r, binary.BigEndian, &p.Pitch)
}

func WriteVarInt(w *bytes.Buffer, val int32) error {
	u := uint32(val)
	for {
		if (u & ^uint32(0x7F)) == 0 {
			w.WriteByte(byte(u))
			return nil
		}
		w.WriteByte(byte((u & 0x7F) | 0x80))
		u >>= 7
	}
}

func ReadVarInt(r *bytes.Buffer) (int32, error) {
	var val uint32
	var cnt int
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		val |= uint32(b&0x7F) << (7 * cnt)
		cnt++
		if (b & 0x80) == 0 {
			break
		}
		if cnt > 5 {
			return 0, fmt.Errorf("VarInt too big")
		}
	}
	return int32(val), nil
}

func WriteString(w *bytes.Buffer, s string) error {
	if err := WriteVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := w.WriteString(s)
	return err
}

func ReadString(r *bytes.Buffer) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	b := make([]byte, length)
	_, err = r.Read(b)
	return string(b), err
}

func ReadVarIntFromReader(r io.Reader) (int32, error) {
	var val uint32
	var cnt int
	var b [1]byte
	for {
		_, err := r.Read(b[:])
		if err != nil {
			return 0, err
		}
		val |= uint32(b[0]&0x7F) << (7 * cnt)
		cnt++
		if (b[0] & 0x80) == 0 {
			break
		}
		if cnt > 5 {
			return 0, fmt.Errorf("VarInt too big")
		}
	}
	return int32(val), nil
}

type PacketPlayerAction struct {
	ActionType int32 // 0: DropOne, 1: DropStack
	Value      int32 // Reserved
}

func (p *PacketPlayerAction) ID() int32 { return IDPlayerAction }
func (p *PacketPlayerAction) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.ActionType)
	return WriteVarInt(w, p.Value)
}
func (p *PacketPlayerAction) Decode(r *bytes.Buffer) error {
	var err error
	p.ActionType, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Value, err = ReadVarInt(r)
	return err
}
