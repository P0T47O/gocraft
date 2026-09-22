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

		s.queueChunkFor(key, username)
	}
}

func (s *Server) queueChunkFor(key chunkKey, user string) {
	// An explicit full request always supersedes a pending light-only refresh.
	delete(s.pendingLights[key], user)
	for _, name := range s.PendingChunks[key] {
		if name == user {
			return
		}
	}
	s.PendingChunks[key] = append(s.PendingChunks[key], user)
}

func chunkPacket(key chunkKey, chunk *Chunk) *PacketChunkData {
	start := time.Now()
	defer func() { perfMon.recordLoading(phaseSnapshot, start) }()
	data := make([]byte, chunkWidth*chunkHeight*chunkWidth)
	light := make([]byte, len(data))
	meta := make([]byte, len(data))
	idx := 0
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				data[idx] = chunk.blocks.Get(x, y, z)
				meta[idx] = chunk.meta.Get(x, y, z)
				light[idx] = (chunk.skyLight.Get(x, y, z) << 4) | (chunk.blockLight.Get(x, y, z) & 15)
				idx++
			}
		}
	}
	return &PacketChunkData{CX: int32(key.X), CZ: int32(key.Z), Data: data, LightData: light, MetaData: meta}
}

func (s *Server) processPendingChunks() {
	deadline := time.Now().Add(2 * time.Millisecond)
	// Border lighting can change already-sent chunks when a neighbor arrives.
	s.ClientsMu.RLock()
	for key := range s.World.lightChanged {
		chunk := s.World.getChunkIfGenerated(key.X, key.Z)
		if chunk == nil {
			continue
		}
		mask := chunk.lightDirtySections
		if mask == 0 {
			mask = 0xffff
		}
		for name, c := range s.Clients {
			if c.KnownChunks[key] {
				s.queueChunkLightFor(key, name, mask)
			}
		}
		chunk.lightDirtySections = 0
	}
	s.ClientsMu.RUnlock()
	clear(s.World.lightChanged)
	if len(s.PendingChunks) == 0 {
		s.chunkOrder = s.chunkOrder[:0]
		return
	}
	if s.chunkOrderNeedsRefresh(time.Now()) {
		s.chunkOrder = s.chunkOrder[:0]
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
		for key := range s.PendingChunks {
			s.chunkOrder = append(s.chunkOrder, chunkPriority{key, distance(key)})
		}
		sort.Slice(s.chunkOrder, func(i, j int) bool {
			di, dj := s.chunkOrder[i].distance, s.chunkOrder[j].distance
			if di != dj {
				return di < dj
			}
			if s.chunkOrder[i].key.X != s.chunkOrder[j].key.X {
				return s.chunkOrder[i].key.X < s.chunkOrder[j].key.X
			}
			return s.chunkOrder[i].key.Z < s.chunkOrder[j].key.Z
		})
		s.ClientsMu.RUnlock()
		s.chunkOrderAt = time.Now()
	}
	limit := streamingBatchSize(len(s.PendingChunks))
	sent := 0
	for _, entry := range s.chunkOrder {
		key := entry.key
		if sent >= limit || time.Now().After(deadline) {
			return
		}
		if len(s.PendingChunks[key]) == 0 {
			continue
		}
		// Prune abandoned requests, and do not allocate snapshots for full peers.
		ready := false
		remaining := s.PendingChunks[key][:0]
		s.ClientsMu.RLock()
		for _, name := range s.PendingChunks[key] {
			c := s.Clients[name]
			if c == nil {
				continue
			}
			dx, dz := key.X-c.LastChunkX, key.Z-c.LastChunkZ
			if dx*dx+dz*dz > (maxRenderDistance+4)*(maxRenderDistance+4) {
				continue
			}
			remaining = append(remaining, name)
			ready = ready || chunkSendHasRoom(c)
		}
		s.ClientsMu.RUnlock()
		if len(remaining) == 0 {
			delete(s.PendingChunks, key)
			delete(s.pendingLights, key)
			continue
		}
		s.PendingChunks[key] = remaining
		if !ready {
			continue
		}
		// Retry submissions previously rejected by a full generation queue.
		chunk := s.World.requestChunk(key.X, key.Z)
		if !chunk.generated {
			continue
		}
		var fullPacket *PacketChunkData
		lightPackets := make(map[uint16]*PacketChunkLight)
		remaining = s.PendingChunks[key][:0]
		s.ClientsMu.Lock()
		for _, name := range s.PendingChunks[key] {
			c := s.Clients[name]
			if c == nil {
				continue
			}
			if !chunkSendHasRoom(c) {
				remaining = append(remaining, name)
				continue
			}
			dx, dz := key.X-c.LastChunkX, key.Z-c.LastChunkZ
			if dx*dx+dz*dz > (maxRenderDistance+4)*(maxRenderDistance+4) {
				continue
			}
			var packet Packet
			if mask := s.pendingLights[key][name]; mask != 0 && c.KnownChunks[key] {
				if lightPackets[mask] == nil {
					lightPackets[mask] = chunkLightPacket(key, chunk, mask)
				}
				packet = lightPackets[mask]
			} else {
				if fullPacket == nil {
					fullPacket = chunkPacket(key, chunk)
				}
				packet = fullPacket
			}
			streamQ := chunkStreamQueue(c)
			select {
			case streamQ <- packet:
				c.KnownChunks[key] = true
				delete(s.pendingLights[key], name)
			default:
				remaining = append(remaining, name)
			}
		}
		s.ClientsMu.Unlock()
		if len(remaining) == 0 {
			delete(s.PendingChunks, key)
			delete(s.pendingLights, key)
		} else {
			s.PendingChunks[key] = remaining
		}
		sent++
	}
}
