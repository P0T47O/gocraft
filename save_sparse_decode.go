package main

import (
	"encoding/binary"
	"errors"
)

// Decode into private planes: a corrupt metadata stream must not partially
// replace the caller's blocks. Version 6 has no metadata stream.
func decodeSparseChunk(palette, data []byte, blocks, meta *chunkPlane, metadata []byte) error {
	if len(palette) == 0 || len(palette) > 256 {
		return errors.New("invalid chunk palette size")
	}
	b, err := decodeSparsePlane(data, palette)
	if err != nil {
		return err
	}
	var m chunkPlane
	if len(metadata) != 0 {
		m, err = decodeSparsePlane(metadata, nil)
		if err != nil {
			return err
		}
	}
	*blocks = b
	if meta != nil {
		*meta = m
	}
	return nil
}

// Runs follow x/y/z wire order, whereas storage is section-local. Split each
// run at a section row boundary and write spans without dense intermediates.
func decodeSparsePlane(data, palette []byte) (chunkPlane, error) {
	var p chunkPlane
	const total = chunkWidth * chunkHeight * chunkWidth
	const row = sectionHeight * chunkWidth
	if len(data)%3 != 0 {
		return p, errors.New("invalid rle length")
	}
	pos := 0
	for i := 0; i < len(data); i += 3 {
		n := int(binary.LittleEndian.Uint16(data[i:]))
		if n == 0 || n > total-pos {
			return chunkPlane{}, errors.New("invalid rle run length")
		}
		v := data[i+2]
		if palette != nil {
			if int(v) >= len(palette) {
				return chunkPlane{}, errors.New("palette index out of range")
			}
			v = palette[v]
		}
		for n > 0 {
			x := pos / (chunkHeight * chunkWidth)
			sec := (pos / row) % sectionCount
			off := pos % row
			count := min(n, row-off)
			s := &p.sections[sec]
			if s.data == nil && s.uniform != v {
				s.data = new([chunkWidth * sectionHeight * chunkWidth]byte)
				for j := range s.data {
					s.data[j] = s.uniform
				}
			}
			if s.data != nil {
				for j := x*row + off; j < x*row+off+count; j++ {
					s.data[j] = v
				}
			}
			pos += count
			n -= count
		}
	}
	if pos != total {
		return chunkPlane{}, errors.New("rle decoded length mismatch")
	}
	p.Compact()
	return p, nil
}
