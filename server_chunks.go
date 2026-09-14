package main

import (
	"sort"
	"time"
)

func (s *Server) SendChunksAround(username string, centerCX, centerCZ, radius int) {
	s.ClientsMu.RLock()
	client, ok := s.Clients[username]
	s.ClientsMu.RUnlock()
	if !ok {
		return
	}

	// Identify missing chunks
	missing := []chunkKey{}
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			cx, cz := centerCX+dx, centerCZ+dz
			key := chunkKey{X: cx, Z: cz}
			if dx*dx+dz*dz > radius*radius {
				continue
			}
			if !client.KnownChunks[key] {
				missing = append(missing, key)
			}
		}
	}

	if len(missing) == 0 {
		return
	}

	// Sort missing chunks by distance to player (Spiral/Center-out)
	sort.Slice(missing, func(i, j int) bool {
		di := (missing[i].X-centerCX)*(missing[i].X-centerCX) + (missing[i].Z-centerCZ)*(missing[i].Z-centerCZ)
		dj := (missing[j].X-centerCX)*(missing[j].X-centerCX) + (missing[j].Z-centerCZ)*(missing[j].Z-centerCZ)
		return di < dj
	})

	for _, key := range missing {
		// Trigger generation priority
		s.World.requestChunk(key.X, key.Z)

		// Add to pending
		if list, ok := s.PendingChunks[key]; ok {
			// Check if already in list to avoid dups
			found := false
			for _, n := range list {
				if n == username {
					found = true
					break
				}
			}
			if !found {
				s.PendingChunks[key] = append(list, username)
			}
		} else {
			s.PendingChunks[key] = []string{username}
		}
	}
}

func (s *Server) queueChunkFor(key chunkKey, user string) {
	for _, name := range s.PendingChunks[key] {
		if name == user {
			return
		}
	}
	s.PendingChunks[key] = append(s.PendingChunks[key], user)
}

func chunkPacket(key chunkKey, chunk *Chunk) *PacketChunkData {
	data := make([]byte, chunkWidth*chunkHeight*chunkWidth)
	light := make([]byte, len(data))
	meta := make([]byte, len(data))
	idx := 0
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				data[idx] = chunk.blocks[x][y][z]
				meta[idx] = chunk.meta[x][y][z]
				light[idx] = (chunk.skyLight[x][y][z] << 4) | (chunk.blockLight[x][y][z] & 15)
				idx++
			}
		}
	}
	return &PacketChunkData{CX: int32(key.X), CZ: int32(key.Z), Data: data, LightData: light, MetaData: meta}
}

func (s *Server) processPendingChunks() {
	// Border lighting can change already-sent chunks when a neighbor arrives.
	s.ClientsMu.RLock()
	for key := range s.World.lightChanged {
		for name, c := range s.Clients {
			if c.KnownChunks[key] {
				s.queueChunkFor(key, name)
			}
		}
	}
	s.ClientsMu.RUnlock()
	clear(s.World.lightChanged)
	keys := make([]chunkKey, 0, len(s.PendingChunks))
	for key := range s.PendingChunks {
		keys = append(keys, key)
	}
	distance := func(key chunkKey) int {
		best := int(^uint(0) >> 1)
		for _, name := range s.PendingChunks[key] {
			if c := s.Clients[name]; c != nil {
				dx, dz := key.X-c.LastChunkX, key.Z-c.LastChunkZ
				if d := dx*dx + dz*dz; d < best {
					best = d
				}
			}
		}
		return best
	}
	s.ClientsMu.RLock()
	sort.Slice(keys, func(i, j int) bool {
		di, dj := distance(keys[i]), distance(keys[j])
		if di != dj {
			return di < dj
		}
		if keys[i].X != keys[j].X {
			return keys[i].X < keys[j].X
		}
		return keys[i].Z < keys[j].Z
	})
	s.ClientsMu.RUnlock()
	deadline := time.Now().Add(2 * time.Millisecond)
	sent := 0
	for _, key := range keys {
		if sent >= 4 || time.Now().After(deadline) {
			return
		}
		// Retry submissions previously rejected by a full generation queue.
		chunk := s.World.requestChunk(key.X, key.Z)
		if !chunk.generated {
			continue
		}
		packet := chunkPacket(key, chunk)
		remaining := s.PendingChunks[key][:0]
		s.ClientsMu.Lock()
		for _, name := range s.PendingChunks[key] {
			c := s.Clients[name]
			if c == nil {
				continue
			}
			dx, dz := key.X-c.LastChunkX, key.Z-c.LastChunkZ
			if dx*dx+dz*dz > (maxRenderDistance+4)*(maxRenderDistance+4) {
				continue
			}
			select {
			case c.Send <- packet:
				c.KnownChunks[key] = true
			default:
				remaining = append(remaining, name)
			}
		}
		s.ClientsMu.Unlock()
		if len(remaining) == 0 {
			delete(s.PendingChunks, key)
		} else {
			s.PendingChunks[key] = remaining
		}
		sent++
	}
}
