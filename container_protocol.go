package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type PacketContainerState struct {
	Token int32
	State BlockContainer
}

func (*PacketContainerState) ID() int32 { return 0x1B }
func (p *PacketContainerState) Encode(w *bytes.Buffer) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return WriteString(w, string(b))
}
func (p *PacketContainerState) Decode(r *bytes.Buffer) error {
	b, err := ReadString(r)
	if err != nil {
		return err
	}
	if len(b) > 8192 {
		return fmt.Errorf("oversized container")
	}
	if err = json.Unmarshal([]byte(b), p); err != nil {
		return err
	}
	if p.State.Kind != 0 && (containerSize(p.State.Kind) == 0 || len(p.State.Slots) != containerSize(p.State.Kind)) {
		return fmt.Errorf("invalid container")
	}
	return nil
}

type PacketContainerClick struct{ Token, Revision, Slot, Button int32 }

func (*PacketContainerClick) ID() int32 { return 0x1C }
func (p *PacketContainerClick) Encode(w *bytes.Buffer) error {
	for _, n := range []int32{p.Token, p.Revision, p.Slot, p.Button} {
		if err := WriteVarInt(w, n); err != nil {
			return err
		}
	}
	return nil
}
func (p *PacketContainerClick) Decode(r *bytes.Buffer) error {
	for _, n := range []*int32{&p.Token, &p.Revision, &p.Slot, &p.Button} {
		v, err := ReadVarInt(r)
		if err != nil {
			return err
		}
		*n = v
	}
	return nil
}
