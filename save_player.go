package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"path/filepath"
)

func SavePlayerState(savePath string, x, y, z float32, selectedSlot int, hotbar []byte, seed uint32) error {
	if err := ensureSaveDir(savePath); err != nil {
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

	path := filepath.Join(savePath, playerFile)
	return writeSaveFile(path, buf.Bytes())
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
