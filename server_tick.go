package main

import (
	"fmt"
	"time"
)

func (s *Server) Tick() {
	if s.Paused.Load() {
		return
	}
	started := time.Now()
	defer func() { perfMon.RecordTick(time.Since(started)) }()
	s.World.ProcessGenResults()
	s.processPendingChunks()
	s.updatePlayerVitals()
	s.UpdateEntities()
	s.updateMobs()
	s.tickContainers()

	// Garbage Collect Chunks
	// Radius 24 (generous buffer)
	// Only if single player or simple check
	// For robust MP: Check against ALL players.

	// Accessing s.Clients requires Lock.
	s.ClientsMu.RLock()
	var playerPositions [][2]int
	for _, c := range s.Clients {
		playerPositions = append(playerPositions, [2]int{c.LastChunkX, c.LastChunkZ})
	}
	s.ClientsMu.RUnlock()

	if len(playerPositions) > 0 {
		// A dead player's camera may be far from their spawn anchor. Generation
		// for safeRespawn must survive GC until the client can finish respawning.
		respawnChunks := s.pendingRespawnChunks()
		// Define unload callback to notify clients (Optional, but good for sync)
		onUnload := func(cx, cz int) {
			s.Broadcast(&PacketUnloadChunk{CX: int32(cx), CZ: int32(cz)})
			key := chunkKey{X: cx, Z: cz}

			// Crucial: Tell server to forget that clients know this chunk.
			// So next time they come close, we resend it.
			s.ClientsMu.Lock()
			for _, c := range s.Clients {
				delete(c.KnownChunks, key)
			}
			s.ClientsMu.Unlock()
		}

		// Custom Unload Logic for Multiplayer support
		// We can't use World.UnloadChunks because it supports only one center.
		// We implement a custom loop here.

		unloadRadiusSq := (maxRenderDistance + 8) * (maxRenderDistance + 8)

		s.World.chunksMu.Lock() // We need Lock to delete
		// Note: Iterating map with Lock is blocking, but necessary for delete.
		// To optimize: RLock first to find candidates, then Lock to delete?
		// Map iteration is safe with RLock? Yes.

		toRemove := make([]chunkKey, 0)
		for key := range s.World.chunks {
			if respawnChunks[key] {
				continue
			}
			// Check if chunk is far from ALL players
			keep := false
			for _, pos := range playerPositions {
				dx := key.X - pos[0]
				dz := key.Z - pos[1]
				if dx*dx+dz*dz <= unloadRadiusSq {
					keep = true
					break
				}
			}

			if !keep {
				// Also check if it's pending?
				// s.PendingChunks contains keys waiting for dispatch.
				// If we unload it, we might break pending logic.
				// PendingChunks keys allow us to keep duplicates?
				// Let's check s.PendingChunks (it's on Server struct, not World).
				// We should not unload if pending.
				// But PendingChunks map usage creates race if not locked?
				// PendingChunks is accessed in Tick (single thread? No, Tick is sequential).
				// Tick -> processPendingChunks. So safe from Tick.
				// But SendChunksAround (from Packet) writes it using ClientsMu? No, SendChunksAround writes PendingChunks.
				// SendChunksAround is called from HandlePacket (channel consumer in Start).
				// Tick is called from Ticker in Start.
				// They are SEQUENTIAL in the main select loop of Start!
				// So PendingChunks access is safe WITHOUT lock.

				isPending := false
				if _, ok := s.PendingChunks[key]; ok {
					isPending = true
				}

				if !isPending {
					toRemove = append(toRemove, key)
				}
			}
		}

		// Delete chunks
		for _, key := range toRemove {
			chunk := s.World.chunks[key]

			// SAVE BEFORE UNLOAD
			if chunk.dirty {
				if err := SaveChunk(s.SavePath, chunk, key.X, key.Z); err != nil {
					fmt.Printf("Error saving chunk %d,%d during unload: %v\n", key.X, key.Z, err)
					// Keep dirty data resident so a later tick/save can retry.
					continue
				}
			}

			delete(s.World.chunks, key)
			delete(s.World.pending, key)
			delete(s.World.pendingEdits, key)
			delete(s.World.lightChanged, key)

			// Safe Free (using our new thread-safe freeChunk)
			// We are holding chunksMu.Lock.
			// freeChunk acquires chunk.mu.Lock.
			// This is safe (Map -> Chunk order).
			// BUT: freeChunk calls w.chunkPool.Put(c).
			// IMPORTANT: We must unlock chunksMu before calling freeChunk?
			// The original UnloadChunks (in world_core.go) calls freeChunk INSIDE chunksMu.Lock loop?
			// Let's check step 309.
			// UnloadChunks: w.chunksMu.Lock(); ... w.freeChunk(chunk); ... w.chunksMu.Unlock().
			// Yes.
			// freeChunk: c.mu.Lock(); ... c.mu.Unlock().
			// So we hold Map Lock, take Chunk Lock.
			// Correct order.

			chunk.mu.Lock()
			s.World.chunkPool.Put(chunk)
			chunk.mu.Unlock()

			if onUnload != nil {
				onUnload(key.X, key.Z)
			}
		}
		s.World.chunksMu.Unlock()
	}
}
