package main

import (
	"encoding/binary"
	"encoding/json"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/klauspost/compress/zstd"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Reusable zstd encoder/decoder to avoid repeated allocation
var (
	zstdEncoder     *zstd.Encoder
	zstdDecoder     *zstd.Decoder
	zstdEncoderOnce sync.Once
	zstdDecoderOnce sync.Once
	zstdEncoderMu   sync.Mutex
	zstdDecoderMu   sync.Mutex
)

func getZstdEncoder() *zstd.Encoder {
	zstdEncoderOnce.Do(func() {
		var err error
		zstdEncoder, err = zstd.NewWriter(nil)
		if err != nil {
			panic("failed to create zstd encoder: " + err.Error())
		}
	})
	return zstdEncoder
}

func getZstdDecoder() *zstd.Decoder {
	zstdDecoderOnce.Do(func() {
		var err error
		zstdDecoder, err = zstd.NewReader(nil)
		if err != nil {
			panic("failed to create zstd decoder: " + err.Error())
		}
	})
	return zstdDecoder
}

const (
	RootSaveDir = "saves"
	chunkDir    = "chunks"
	playerFile  = "player.bin"
	levelFile   = "level.json"
	chunkMagic  = "GCS1"
	playerMagic = "GCP1"
	saveVersion = 7
	entityFile  = "entities.bin"
	entityMagic = "GCE1"
)

type LevelData struct {
	Name       string `json:"name"`
	Seed       int64  `json:"seed"`
	LastPlayed int64  `json:"last_played"`
	Version    int    `json:"version"`
}

const (
	saveFlagPalette = 1 << iota
	saveFlagRLE
)

// ensureSaveDir creates the save directory if it doesn't exist
func ensureSaveDir(savePath string) error {
	return os.MkdirAll(savePath, 0o755)
}

func SaveGame(savePath string, world *World, state *InputState, camera rl.Camera3D) error {
	if err := ensureSaveDir(savePath); err != nil {
		return err
	}

	// Save Level Metadata
	if err := SaveLevelData(savePath, world.seed); err != nil {
		return err
	}

	if err := SaveWorldChunks(savePath, world); err != nil {
		return err
	}
	if err := SaveEntities(savePath, world); err != nil {
		return err
	}
	if err := SavePlayerState(savePath, camera.Position.X, camera.Position.Y, camera.Position.Z, state.SelectedSlot, state.Hotbar[:], world.seed); err != nil {
		return err
	}
	return nil
}

func SaveLevelData(savePath string, seed uint32) error {
	path := filepath.Join(savePath, levelFile)

	// Try to read existing to keep created time or name if we had one
	data := LevelData{
		Name:       filepath.Base(savePath),
		Seed:       int64(seed),
		LastPlayed: time.Now().Unix(),
		Version:    saveVersion,
	}

	// Perform the save
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return writeSaveFile(path, bytes)
}

func LoadGame(savePath string, world *World, state *InputState, camera *rl.Camera3D) error {
	if err := loadAllChunks(savePath, world); err != nil {
		return err
	}
	if err := loadPlayerFile(savePath, state, camera, world); err != nil {
		return err
	}

	world.ClearDirty()
	return nil
}

func LoadWorld(savePath string, world *World) (bool, float64, float64, float64, error) {
	// New worlds have level.json before they have player/chunk files.
	if data, err := os.ReadFile(filepath.Join(savePath, levelFile)); err == nil {
		var level LevelData
		if err := json.Unmarshal(data, &level); err != nil {
			return false, 0, 0, 0, err
		}
		world.seed = uint32(level.Seed)
	}

	var posX, posY, posZ float64
	hasPos := false

	// 1. Load Seed and Pos from player file (it's stored there for now)
	playerPath := filepath.Join(savePath, playerFile)
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

	// Chunks are loaded on demand by generation workers, not all at startup.
	world.SavePath = savePath
	// 3. Load Entities
	if _, err := LoadEntities(savePath, world); err != nil && !os.IsNotExist(err) {
		return false, 0, 0, 0, err
	}

	return hasPos, posX, posY, posZ, nil
}

func LoadGameIfExists(savePath string, world *World, state *InputState, camera *rl.Camera3D) (bool, bool, error) {
	playerPath := filepath.Join(savePath, playerFile)
	chunkExists := false
	if _, err := os.Stat(filepath.Join(savePath, chunkDir)); err == nil {
		chunkFiles, _ := filepath.Glob(filepath.Join(savePath, chunkDir, "*.bin"))
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
		if err := loadAllChunks(savePath, world); err != nil {
			return false, false, err
		}
	}
	if playerExists {
		if err := loadPlayerFile(savePath, state, camera, world); err != nil {
			return false, false, err
		}

	}
	world.ClearDirty()
	return chunkExists, playerExists, nil
}
