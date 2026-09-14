package main

import (
	"bytes"
	"io"
	"testing"
)

func TestVarIntReaders(t *testing.T) {
	readers := map[string]func(*bytes.Buffer) (int32, error){
		"buffer": ReadVarInt,
		"stream": func(b *bytes.Buffer) (int32, error) { return ReadVarIntFromReader(b) },
	}
	for name, read := range readers {
		t.Run(name, func(t *testing.T) {
			for _, v := range []int32{0, 1, 127, 128, 16383, 16384, 2147483647, -2147483648, -1} {
				var b bytes.Buffer
				WriteVarInt(&b, v)
				b.WriteByte(42)
				got, err := read(&b)
				if err != nil || got != v || b.Len() != 1 {
					t.Fatalf("%d: got %d, %v, remaining %d", v, got, err, b.Len())
				}
			}
			for _, data := range [][]byte{nil, {0x80}, {0xff, 0xff}, {0x80, 0x80, 0x80, 0x80, 0x10}, {0x80, 0x80, 0x80, 0x80, 0x80, 1}, {0xff, 0xff, 0xff, 0xff, 0xff, 0}} {
				b := bytes.NewBuffer(data)
				if _, err := read(b); err == nil {
					t.Fatalf("accepted malformed encoding %x", data)
				}
				if len(data) > 5 && b.Len() != len(data)-5 {
					t.Fatal("reader consumed beyond the five-byte limit")
				}
			}
		})
	}
}

type emptyFirstReader struct {
	empty bool
	r     io.Reader
}

func (r *emptyFirstReader) Read(p []byte) (int, error) {
	if !r.empty {
		r.empty = true
		return 0, nil
	}
	return r.r.Read(p)
}

func TestVarIntStreamEmptyRead(t *testing.T) {
	r := &emptyFirstReader{r: bytes.NewReader([]byte{0xac, 2})}
	if got, err := ReadVarIntFromReader(r); err != nil || got != 300 {
		t.Fatalf("got %d, %v", got, err)
	}
}

func TestReadStringRejectsTruncation(t *testing.T) {
	for _, data := range [][]byte{{3, 'a'}, {1}, {0xff, 0xff, 0xff, 0xff, 0x0f}} {
		if _, err := ReadString(bytes.NewBuffer(data)); err == nil {
			t.Fatalf("accepted %x", data)
		}
	}
	if s, err := ReadString(bytes.NewBuffer([]byte{0})); err != nil || s != "" {
		t.Fatalf("empty string: %q, %v", s, err)
	}
}
