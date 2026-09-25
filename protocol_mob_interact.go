package main

import "bytes"

type PacketInteractMob struct{ Target string }

func (*PacketInteractMob) ID() int32                      { return IDInteractMob }
func (p *PacketInteractMob) Encode(w *bytes.Buffer) error { return WriteString(w, p.Target) }
func (p *PacketInteractMob) Decode(r *bytes.Buffer) error {
	var err error
	p.Target, err = ReadString(r)
	return err
}
