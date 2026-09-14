package main

import (
	"fmt"
)

func (s *Server) Save() {
	fmt.Println("Server: Saving world state...")
	if err := s.saveContainers(); err != nil {
		fmt.Printf("Container save failed: %v\n", err)
	}
	if err := saveSurvivalPlayers(s.SavePath, s.World); err != nil {
		fmt.Printf("Player save failed: %v\n", err)
	}
	// 1. Save Chunks
	if err := SaveWorldChunks(s.SavePath, s.World); err != nil {
		fmt.Printf("Server Save Chunks Error: %v\n", err)
	}
	// 2. Save Entities
	if err := SaveEntities(s.SavePath, s.World); err != nil {
		fmt.Printf("Server Save Entities Error: %v\n", err)
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
			fmt.Printf("Server Save Player Error: %v\n", err)
		}
	} else {
		// Fallback if no player entity found (e.g. just started server and quit)
		if err := SavePlayerState(s.SavePath, float32(s.InitialPosX), float32(s.InitialPosY), float32(s.InitialPosZ), 0, make([]byte, 9), s.World.seed); err != nil {
			fmt.Printf("Server Save Player Fallback Error: %v\n", err)
		}
	}
	fmt.Println("Server: World saved successfully.")
}
