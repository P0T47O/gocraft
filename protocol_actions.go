package main

import (
	"bytes"
	"encoding/binary"
)

type PacketClickWindow struct {
	SlotID     int32
	Button     int32 // 0: Left, 1: Right, 2: Shift transfer
	IsCreative bool  // Whether source is creative palette (for server logic)
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
	Action  int32 // 0: Interact, 1: Begin mining, 2: Cancel mining
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
	Count    int32 // Batches, 0 means one for legacy clients; bounded to 64.
}

func (p *PacketCraft) ID() int32 { return IDCraft }
func (p *PacketCraft) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.RecipeID)
	return WriteVarInt(w, p.Count)
}
func (p *PacketCraft) Decode(r *bytes.Buffer) error {
	var err error
	p.RecipeID, err = ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Count = 0
	if r.Len() > 0 {
		p.Count, err = ReadVarInt(r)
	}
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
	SlotID int32 // 0-8 hotbar, 9-35 backpack, 36-39 armor
	ItemID int32
	Count  int32
	Damage int32
}

func (p *PacketInventoryUpdate) ID() int32 { return IDInventoryUpdate }
func (p *PacketInventoryUpdate) Encode(w *bytes.Buffer) error {
	WriteVarInt(w, p.SlotID)
	WriteVarInt(w, p.ItemID)
	WriteVarInt(w, p.Count)
	WriteVarInt(w, p.Damage)
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
	p.Damage, err = ReadVarInt(r)
	return err
}

type PacketPlayerAction struct {
	ActionType int32 // 0: DropOne, 2: Eat, 3: Shoot selected bow
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
