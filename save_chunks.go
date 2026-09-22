package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func SaveWorldChunks(savePath string, world *World) error {
	chunkPath := filepath.Join(savePath, chunkDir)
	if err := os.MkdirAll(chunkPath, 0o755); err != nil {
		return err
	}
	// Note: We don't ClearDirty here yet because we might want multiple clients to save.
	// But in singleplayer, server clears it.
	world.chunksMu.RLock()
	keys := make([]chunkKey, 0, len(world.chunks))
	for key := range world.chunks {
		keys = append(keys, key)
	}
	world.chunksMu.RUnlock()

	for _, key := range keys {
		world.chunksMu.RLock()
		chunk := world.chunks[key]
		world.chunksMu.RUnlock()
		if chunk == nil || !chunk.dirty {
			continue
		}
		if err := saveChunkFile(savePath, key.X, 0, key.Z, &chunk.blocks, &chunk.meta); err != nil {
			return err
		}
		chunk.dirty = false
	}
	world.ClearDirty()
	return nil
}

func SaveChunk(savePath string, chunk *Chunk, chunkX, chunkZ int) error {
	chunkPath := filepath.Join(savePath, chunkDir)
	if err := os.MkdirAll(chunkPath, 0o755); err != nil {
		return err
	}
	if err := saveChunkFile(savePath, chunkX, 0, chunkZ, &chunk.blocks, &chunk.meta); err != nil {
		return err
	}
	chunk.dirty = false
	return nil
}

// TryLoadChunk attempts to load a chunk from disk. A missing file is the only
// condition that permits procedural generation. Existing but unreadable or
// corrupt data is a fatal integrity error so it cannot be silently overwritten.
func TryLoadChunk(savePath string, chunk *Chunk, chunkX, chunkZ int) bool {
	err := loadChunkFile(savePath, chunkX, 0, chunkZ, &chunk.blocks, &chunk.meta)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		panic(fmt.Errorf("load chunk %d,%d from %q: %w", chunkX, chunkZ, savePath, err))
	}
	// Successfully loaded from disk
	chunk.generated = true
	chunk.dirty = false // Just loaded, not modified yet
	return true
}

func saveChunkFile(root string, x, y, z int, blocks, meta *chunkPlane) error {
	palette, rleBlocks, rleMeta := encodeSparseChunk(blocks, meta)
	payload := make([]byte, 0, 2+len(palette)+4+len(rleBlocks)+4+len(rleMeta))
	payload = appendUint16(payload, uint16(len(palette)))
	payload = append(payload, palette...)
	payload = appendUint32(payload, uint32(len(rleBlocks)))
	payload = append(payload, rleBlocks...)
	payload = appendUint32(payload, uint32(len(rleMeta)))
	payload = append(payload, rleMeta...)

	zstdEncoderMu.Lock()
	enc := getZstdEncoder()
	compressed := enc.EncodeAll(payload, nil)
	zstdEncoderMu.Unlock()

	var header bytes.Buffer
	header.WriteString(chunkMagic)
	header.WriteByte(saveVersion)
	writeInt32(&header, int32(x))
	writeInt32(&header, int32(y))
	writeInt32(&header, int32(z))
	header.WriteByte(saveFlagPalette | saveFlagRLE)
	writeUint32(&header, uint32(len(payload)))

	path := filepath.Join(root, chunkDir, fmt.Sprintf("%d_%d_%d.bin", x, y, z))
	return writeSaveFile(path, append(header.Bytes(), compressed...))
}

func loadChunkFile(root string, x, y, z int, blocks, meta *chunkPlane) error {
	path := filepath.Join(root, chunkDir, fmt.Sprintf("%d_%d_%d.bin", x, y, z))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 4+1+4+4+4+1+4 {
		return errors.New("chunk save too small")
	}
	if string(data[:4]) != chunkMagic {
		return errors.New("chunk save magic mismatch")
	}
	version := data[4]
	if version != saveVersion && version != 6 {
		return errors.New("chunk save version mismatch")
	}
	offset := 5
	readInt32 := func() int32 {
		v := int32(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		return v
	}
	cx := readInt32()
	cy := readInt32()
	cz := readInt32()
	if int(cx) != x || int(cy) != y || int(cz) != z {
		return errors.New("chunk coords mismatch")
	}
	flags := data[offset]
	offset++
	if flags&saveFlagPalette == 0 || flags&saveFlagRLE == 0 {
		return errors.New("chunk save flags unsupported")
	}
	uncompressedLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	zstdDecoderMu.Lock()
	dec := getZstdDecoder()
	payload, err := dec.DecodeAll(data[offset:], nil)
	zstdDecoderMu.Unlock()
	if err != nil {
		return err
	}
	if uint32(len(payload)) != uncompressedLen {
		return errors.New("chunk payload size mismatch")
	}
	if len(payload) < 2 {
		return errors.New("chunk payload too small")
	}
	palLen := int(binary.LittleEndian.Uint16(payload[:2]))
	pos := 2
	if len(payload) < pos+palLen+4 {
		return errors.New("chunk palette too small")
	}
	palette := payload[pos : pos+palLen]
	pos += palLen
	rleLen := int(binary.LittleEndian.Uint32(payload[pos:]))
	pos += 4
	if len(payload) < pos+rleLen {
		return errors.New("chunk rle too small")
	}
	rleBlocks := payload[pos : pos+rleLen]
	pos += rleLen
	if version == 6 {
		return decodeSparseChunk(palette, rleBlocks, blocks, meta, nil)
	}
	if len(payload) < pos+4 {
		return errors.New("chunk meta header too small")
	}
	metaLen := int(binary.LittleEndian.Uint32(payload[pos:]))
	pos += 4
	if len(payload) < pos+metaLen {
		return errors.New("chunk meta too small")
	}
	rleMeta := payload[pos : pos+metaLen]
	return decodeSparseChunk(palette, rleBlocks, blocks, meta, rleMeta)
}

func loadAllChunks(root string, world *World) error {
	files, err := filepath.Glob(filepath.Join(root, chunkDir, "*.bin"))
	if err != nil {
		return err
	}
	for _, path := range files {
		var x, y, z int
		n, err := fmt.Sscanf(filepath.Base(path), "%d_%d_%d.bin", &x, &y, &z)
		if err != nil || n != 3 {
			continue
		}
		chunk := world.ensureChunk(x, z)
		if err := loadChunkFile(root, x, y, z, &chunk.blocks, &chunk.meta); err != nil {
			return err
		}
		chunk.rebuildHeightMap()
		chunk.rebuildTorchCount()
		ensureChunkSections(chunk)
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
			chunk.meshVersion[i]++
		}
		chunk.generated = true
		chunk.dirty = false
		delete(world.pending, chunkKey{X: x, Z: z})
		world.rebuildLightingForChunk(x, z)
	}
	return nil
}
