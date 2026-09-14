package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

func ReadUint32(r *bytes.Buffer) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func encodeChunk(blocks *[chunkWidth][chunkHeight][chunkWidth]byte, meta *[chunkWidth][chunkHeight][chunkWidth]byte) ([]byte, []byte, []byte) {
	palette := make([]byte, 0, 32)
	indexMap := map[byte]byte{}
	indices := make([]byte, 0, chunkWidth*chunkHeight*chunkWidth)
	metaValues := make([]byte, 0, chunkWidth*chunkHeight*chunkWidth)

	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				id := blocks[x][y][z]
				idx, ok := indexMap[id]
				if !ok {
					idx = byte(len(palette))
					indexMap[id] = idx
					palette = append(palette, id)
				}
				indices = append(indices, idx)
				if meta != nil {
					metaValues = append(metaValues, meta[x][y][z])
				} else {
					metaValues = append(metaValues, 0)
				}
			}
		}
	}
	return palette, rleEncode(indices), rleEncode(metaValues)
}

func decodeChunk(palette []byte, rle []byte, blocks *[chunkWidth][chunkHeight][chunkWidth]byte, meta *[chunkWidth][chunkHeight][chunkWidth]byte, rleMeta []byte) error {
	indices, err := rleDecode(rle, chunkWidth*chunkHeight*chunkWidth)
	if err != nil {
		return err
	}
	if len(indices) != chunkWidth*chunkHeight*chunkWidth {
		return errors.New("decoded index count mismatch")
	}
	metaValues := make([]byte, chunkWidth*chunkHeight*chunkWidth)
	if rleMeta != nil {
		metaValues, err = rleDecode(rleMeta, chunkWidth*chunkHeight*chunkWidth)
		if err != nil {
			return err
		}
	}
	i := 0
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				idx := int(indices[i])
				if idx < 0 || idx >= len(palette) {
					return errors.New("palette index out of range")
				}
				blocks[x][y][z] = palette[idx]
				if meta != nil {
					meta[x][y][z] = metaValues[i]
				}
				i++
			}
		}
	}
	return nil
}

func rleEncode(values []byte) []byte {
	if len(values) == 0 {
		return nil
	}
	out := make([]byte, 0, len(values))
	runVal := values[0]
	runLen := 1
	flush := func() {
		out = appendUint16(out, uint16(runLen))
		out = append(out, runVal)
	}
	for i := 1; i < len(values); i++ {
		v := values[i]
		if v == runVal && runLen < 0xffff {
			runLen++
			continue
		}
		flush()
		runVal = v
		runLen = 1
	}
	flush()
	return out
}

func rleDecode(data []byte, expected int) ([]byte, error) {
	out := make([]byte, 0, expected)
	if len(data)%3 != 0 {
		return nil, errors.New("invalid rle length")
	}
	for i := 0; i < len(data); i += 3 {
		runLen := int(binary.LittleEndian.Uint16(data[i:]))
		if runLen <= 0 {
			return nil, errors.New("invalid rle run length")
		}
		val := data[i+2]
		for j := 0; j < runLen; j++ {
			out = append(out, val)
		}
	}
	if expected > 0 && len(out) != expected {
		return nil, errors.New("rle decoded length mismatch")
	}
	return out, nil
}

func appendUint16(dst []byte, v uint16) []byte {
	buf := []byte{0, 0}
	binary.LittleEndian.PutUint16(buf, v)
	return append(dst, buf...)
}

func appendUint32(dst []byte, v uint32) []byte {
	buf := []byte{0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf, v)
	return append(dst, buf...)
}

func writeUint32(buf *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	buf.Write(tmp[:])
}

func writeInt32(buf *bytes.Buffer, v int32) {
	writeUint32(buf, uint32(v))
}

func writeFloat32(buf *bytes.Buffer, v float32) {
	writeUint32(buf, math.Float32bits(v))
}

func readFloat32(data []byte) float32 {
	if len(data) < 4 {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(data[:4]))
}
