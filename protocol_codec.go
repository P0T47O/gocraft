package main

import (
	"bytes"
	"fmt"
	"io"
)

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
	return readVarInt(r.ReadByte)
}

func readVarInt(readByte func() (byte, error)) (int32, error) {
	var val uint32
	for cnt := 0; cnt < 5; cnt++ {
		b, err := readByte()
		if err != nil {
			return 0, err
		}
		// Only four payload bits remain in the fifth byte. Check BEFORE
		// shifting, including the continuation bit, so overflow cannot vanish.
		if cnt == 4 && b&0xF0 != 0 {
			return 0, fmt.Errorf("VarInt too big")
		}
		val |= uint32(b&0x7F) << (7 * cnt)
		if (b & 0x80) == 0 {
			return int32(val), nil
		}
	}
	return 0, fmt.Errorf("VarInt too big")
}

func WriteString(w *bytes.Buffer, s string) error {
	if err := WriteVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := w.WriteString(s)
	return err
}

const maxStringLength = 32768 // 32 KB max string length

func ReadString(r *bytes.Buffer) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 || length > maxStringLength {
		return "", fmt.Errorf("string length out of range: %d", length)
	}
	b := make([]byte, length)
	_, err = io.ReadFull(r, b)
	return string(b), err
}

func ReadVarIntFromReader(r io.Reader) (int32, error) {
	var b [1]byte
	return readVarInt(func() (byte, error) {
		_, err := io.ReadFull(r, b[:])
		return b[0], err
	})
}
