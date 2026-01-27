package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/klauspost/compress/zstd"
)

const (
	saveDir     = "saves"
	chunkDir    = "chunks"
	playerFile  = "player.bin"
	chunkMagic  = "GCS1"
	playerMagic = "GCP1"
	saveVersion = 7
	entityFile  = "entities.bin"
	entityMagic = "GCE1"
)

const (
	saveFlagPalette = 1 << iota
	saveFlagRLE
)

func SaveGame(world *World, state *InputState, camera rl.Camera3D) error {
	if err := SaveWorldChunks(world); err != nil {
		return err
	}
	if err := SaveEntities(world); err != nil {
		return err
	}
	if err := SavePlayerState(camera.Position.X, camera.Position.Y, camera.Position.Z, state.SelectedSlot, state.Hotbar[:], world.seed); err != nil {
		return err
	}
	return nil
}

func SaveEntities(world *World) error {
	root := filepath.Join(saveDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString(entityMagic)
	buf.WriteByte(saveVersion)

	world.entitiesMu.RLock()
	defer world.entitiesMu.RUnlock()

	writeUint32(&buf, uint32(len(world.entities)))
	for _, e := range world.entities {
		x, y, z := e.GetPosition()
		yaw, pitch := e.GetRotation()

		_ = WriteString(&buf, e.GetUUID())
		buf.WriteByte(byte(e.GetType()))
		_ = binary.Write(&buf, binary.LittleEndian, x)
		_ = binary.Write(&buf, binary.LittleEndian, y)
		_ = binary.Write(&buf, binary.LittleEndian, z)
		_ = binary.Write(&buf, binary.LittleEndian, yaw)
		_ = binary.Write(&buf, binary.LittleEndian, pitch)
	}

	path := filepath.Join(root, entityFile)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func SavePlayerState(x, y, z float32, selectedSlot int, hotbar []byte, seed uint32) error {
	root := filepath.Join(saveDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(playerMagic)
	buf.WriteByte(saveVersion)
	buf.WriteByte(byte(selectedSlot))
	buf.WriteByte(byte(len(hotbar)))
	buf.Write(hotbar)
	writeFloat32(&buf, x)
	writeFloat32(&buf, y)
	writeFloat32(&buf, z)
	writeUint32(&buf, seed)

	path := filepath.Join(root, playerFile)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func SaveWorldChunks(world *World) error {
	root := filepath.Join(saveDir)
	if err := os.MkdirAll(filepath.Join(root, chunkDir), 0o755); err != nil {
		return err
	}
	// Note: We don't ClearDirty here yet because we might want multiple clients to save.
	// But in singleplayer, server clears it.
	for key, chunk := range world.chunks {
		if !chunk.dirty {
			continue
		}
		if err := saveChunkFile(root, key.X, 0, key.Z, &chunk.blocks, &chunk.meta); err != nil {
			return err
		}
		chunk.dirty = false
	}
	world.ClearDirty()
	return nil
}

func SaveChunk(world *World, chunk *Chunk, chunkX, chunkZ int) error {
	root := filepath.Join(saveDir)
	if err := os.MkdirAll(filepath.Join(root, chunkDir), 0o755); err != nil {
		return err
	}
	if err := saveChunkFile(root, chunkX, 0, chunkZ, &chunk.blocks, &chunk.meta); err != nil {
		return err
	}
	chunk.dirty = false
	return nil
}

func LoadGame(world *World, state *InputState, camera *rl.Camera3D) error {
	root := filepath.Join(saveDir)
	if err := loadAllChunks(root, world); err != nil {
		return err
	}
	if err := loadPlayerFile(root, state, camera, world); err != nil {
		return err
	}
	state.Dragging = false
	world.ClearDirty()
	return nil
}

func LoadWorld(world *World) (bool, float64, float64, float64, error) {
	root := filepath.Join(saveDir)
	if _, err := os.Stat(filepath.Join(root, chunkDir)); err != nil {
		return false, 0, 0, 0, nil // No save exists
	}

	var posX, posY, posZ float64
	hasPos := false

	// 1. Load Seed and Pos from player file (it's stored there for now)
	playerPath := filepath.Join(root, playerFile)
	if data, err := os.ReadFile(playerPath); err == nil && len(data) >= 4+1+1+1+12+4 {
		// Version 7: [Magic:4][Ver:1][Selected:1][HotbarLen:1][Hotbar:9][PosX:4][PosY:4][PosZ:4][Seed:4]
		hotbarLen := int(data[6])
		posStart := 7 + hotbarLen
		if len(data) >= posStart+12+4 {
			posX = float64(readFloat32(data[posStart:]))
			posY = float64(readFloat32(data[posStart+4:]))
			posZ = float64(readFloat32(data[posStart+8:]))
			hasPos = true

			seedStart := posStart + 12
			world.seed = binary.LittleEndian.Uint32(data[seedStart:])
		}
	}

	// 2. Load Chunks
	if err := loadAllChunks(root, world); err != nil {
		return false, 0, 0, 0, err
	}
	// 3. Load Entities
	_, _ = LoadEntities(world)

	return hasPos, posX, posY, posZ, nil
}

func LoadEntities(world *World) (bool, error) {
	root := filepath.Join(saveDir)
	path := filepath.Join(root, entityFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(data) < 4+1+4 {
		return false, errors.New("entity save too small")
	}
	if string(data[:4]) != entityMagic {
		return false, errors.New("entity save magic mismatch")
	}

	buf := bytes.NewBuffer(data[5:])
	count, _ := ReadUint32(buf)

	world.entitiesMu.Lock()
	defer world.entitiesMu.Unlock()
	world.entities = nil // Clear existing entities for full reload

	for i := uint32(0); i < count; i++ {
		uuid, _ := ReadString(buf)
		etype, _ := buf.ReadByte()
		var x, y, z float64
		var yaw, pitch float32
		_ = binary.Read(buf, binary.LittleEndian, &x)
		_ = binary.Read(buf, binary.LittleEndian, &y)
		_ = binary.Read(buf, binary.LittleEndian, &z)
		_ = binary.Read(buf, binary.LittleEndian, &yaw)
		_ = binary.Read(buf, binary.LittleEndian, &pitch)

		// Entity Factory
		var e Entity
		switch EntityType(etype) {
		case EntityPig:
			e = &PigEntity{
				BaseEntity: BaseEntity{
					UUID: uuid, Type: EntityType(etype),
					X: x, Y: y, Z: z,
					Yaw: yaw, Pitch: pitch,
				},
			}
		case EntityPlayer:
			e = &PlayerEntity{
				BaseEntity: BaseEntity{
					UUID: uuid, Type: EntityType(etype),
					X: x, Y: y, Z: z,
					Yaw: yaw, Pitch: pitch,
				},
			}
		default:
			e = &BaseEntity{
				UUID: uuid, Type: EntityType(etype),
				X: x, Y: y, Z: z,
				Yaw: yaw, Pitch: pitch,
			}
		}
		world.entities = append(world.entities, e)
	}

	return true, nil
}

func ReadUint32(r *bytes.Buffer) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func LoadGameIfExists(world *World, state *InputState, camera *rl.Camera3D) (bool, bool, error) {
	root := filepath.Join(saveDir)
	playerPath := filepath.Join(root, playerFile)
	chunkExists := false
	if _, err := os.Stat(filepath.Join(root, chunkDir)); err == nil {
		chunkFiles, _ := filepath.Glob(filepath.Join(root, chunkDir, "*.bin"))
		chunkExists = len(chunkFiles) > 0
	}
	playerExists := true
	if _, err := os.Stat(playerPath); err != nil {
		if os.IsNotExist(err) {
			playerExists = false
		} else {
			return false, false, err
		}
	}
	if !chunkExists && !playerExists {
		return false, false, nil
	}
	if chunkExists {
		if err := loadAllChunks(root, world); err != nil {
			return false, false, err
		}
	}
	if playerExists {
		if err := loadPlayerFile(root, state, camera, world); err != nil {
			return false, false, err
		}
		state.Dragging = false
	}
	world.ClearDirty()
	return chunkExists, playerExists, nil
}

func saveChunkFile(root string, x, y, z int, blocks *[chunkWidth][chunkHeight][chunkWidth]byte, meta *[chunkWidth][chunkHeight][chunkWidth]byte) error {
	palette, rleBlocks, rleMeta := encodeChunk(blocks, meta)
	payload := make([]byte, 0, 2+len(palette)+4+len(rleBlocks)+4+len(rleMeta))
	payload = appendUint16(payload, uint16(len(palette)))
	payload = append(payload, palette...)
	payload = appendUint32(payload, uint32(len(rleBlocks)))
	payload = append(payload, rleBlocks...)
	payload = appendUint32(payload, uint32(len(rleMeta)))
	payload = append(payload, rleMeta...)

	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return err
	}
	compressed := enc.EncodeAll(payload, nil)
	enc.Close()

	var header bytes.Buffer
	header.WriteString(chunkMagic)
	header.WriteByte(saveVersion)
	writeInt32(&header, int32(x))
	writeInt32(&header, int32(y))
	writeInt32(&header, int32(z))
	header.WriteByte(saveFlagPalette | saveFlagRLE)
	writeUint32(&header, uint32(len(payload)))

	path := filepath.Join(root, chunkDir, fmt.Sprintf("%d_%d_%d.bin", x, y, z))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(header.Bytes()); err != nil {
		return err
	}
	_, err = f.Write(compressed)
	return err
}

func loadChunkFile(root string, x, y, z int, blocks *[chunkWidth][chunkHeight][chunkWidth]byte, meta *[chunkWidth][chunkHeight][chunkWidth]byte) error {
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

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	payload, err := dec.DecodeAll(data[offset:], nil)
	dec.Close()
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
		return decodeChunk(palette, rleBlocks, blocks, meta, nil)
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
	return decodeChunk(palette, rleBlocks, blocks, meta, rleMeta)
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

func loadPlayerFile(root string, state *InputState, camera *rl.Camera3D, world *World) error {
	path := filepath.Join(root, playerFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 4+1+1+1+12+4 {
		return errors.New("player save too small")
	}
	if string(data[:4]) != playerMagic {
		return errors.New("player save magic mismatch")
	}
	if data[4] != saveVersion {
		return errors.New("player save version mismatch")
	}
	selected := int(data[5])
	hotbarLen := int(data[6])
	if hotbarLen != len(state.Hotbar) {
		return errors.New("player hotbar size mismatch")
	}
	if len(data) < 7+hotbarLen {
		return errors.New("player save truncated")
	}
	copy(state.Hotbar[:], data[7:7+hotbarLen])
	if selected >= 0 && selected < len(state.Hotbar) {
		state.SelectedSlot = selected
		state.CurrentBlock = state.Hotbar[state.SelectedSlot]
	}
	posStart := 7 + hotbarLen
	camera.Position.X = readFloat32(data[posStart:])
	camera.Position.Y = readFloat32(data[posStart+4:])
	camera.Position.Z = readFloat32(data[posStart+8:])
	seedStart := posStart + 12
	world.seed = binary.LittleEndian.Uint32(data[seedStart:])
	return nil
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
