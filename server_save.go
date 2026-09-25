package main

import (
	"errors"
	"fmt"
)

func (s *Server) Save() error {
	var failures []error
	record := func(stage string, err error) { failures = append(failures, fmt.Errorf("%s: %w", stage, err)) }
	fmt.Println("Server: Saving world state...")
	if err := SaveLevelState(s.SavePath, s.World.seed, int64(s.World.TimeTicks)); err != nil {
		record("World clock save failed", err)
	}
	if err := s.saveContainers(); err != nil {
		record("Container save failed", err)
	}
	if err := saveSurvivalPlayers(s.SavePath, s.World); err != nil {
		record("Player save failed", err)
	}
	// 1. Save Chunks
	if err := SaveWorldChunks(s.SavePath, s.World); err != nil {
		record("Server Save Chunks Error", err)
	}
	// 2. Save Entities
	if err := SaveEntities(s.SavePath, s.World); err != nil {
		record("Server Save Entities Error", err)
	}
	// 3. Save Player (Authoritative)
	// Find the player entity to save
	s.World.entitiesMu.RLock()
	var player *PlayerEntity
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok {
			// For singleplayer/host, we pick the first player or specific one if we knew the name.
			// Currently we save to 'player.bin' which implies single player.
			player = p
			break
		}
	}
	s.World.entitiesMu.RUnlock()

	if player != nil {
		// Serialize Hotbar
		hotbarBytes := make([]byte, 9)
		for i := 0; i < 9; i++ {
			hotbarBytes[i] = byte(player.Inventory.Slots[i].ID)
		}

		fmt.Printf("Saving Player: %s at %.2f, %.2f, %.2f (Seed: %d)\n", player.UUID, player.X, player.Y, player.Z, s.World.seed)
		if err := SavePlayerState(s.SavePath, float32(player.X), float32(player.Y), float32(player.Z), player.SelectedSlot, hotbarBytes, s.World.seed); err != nil {
			record("Server Save Player Error", err)
		}
	} else {
		// Fallback if no player entity found (e.g. just started server and quit)
		if err := SavePlayerState(s.SavePath, float32(s.InitialPosX), float32(s.InitialPosY), float32(s.InitialPosZ), 0, make([]byte, 9), s.World.seed); err != nil {
			record("Server Save Player Fallback Error", err)
		}
	}
	if err := errors.Join(failures...); err != nil {
		fmt.Printf("Server: World save FAILED: %v\n", err)
		return err
	}
	fmt.Println("Server: World saved successfully.")
	return nil
}
