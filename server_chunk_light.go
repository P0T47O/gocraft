package main

import "time"

func (s *Server) queueChunkLightFor(key chunkKey, user string, mask uint16) {
	for _, name := range s.PendingChunks[key] {
		if name == user {
			if s.pendingLights[key][user] != 0 {
				s.pendingLights[key][user] |= mask
			}
			return // A full snapshot already pending includes the latest light.
		}
	}
	if s.pendingLights == nil {
		s.pendingLights = make(map[chunkKey]map[string]uint16)
	}
	if s.pendingLights[key] == nil {
		s.pendingLights[key] = make(map[string]uint16)
	}
	s.pendingLights[key][user] = mask
	s.PendingChunks[key] = append(s.PendingChunks[key], user)
}

func chunkLightPacket(key chunkKey, c *Chunk, mask uint16) *PacketChunkLight {
	start := time.Now()
	defer func() { perfMon.recordLoading(phaseSnapshot, start) }()
	p := &PacketChunkLight{CX: int32(key.X), CZ: int32(key.Z), Sections: mask, Data: make([]byte, chunkLightSize(mask))}
	i := 0
	for sec := 0; sec < sectionCount; sec++ {
		if mask&(1<<sec) != 0 {
			for x := 0; x < chunkWidth; x++ {
				for y := sec * sectionHeight; y < (sec+1)*sectionHeight; y++ {
					for z := 0; z < chunkWidth; z++ {
						p.Data[i] = c.skyLight.Get(x, y, z)<<4 | c.blockLight.Get(x, y, z)&15
						i++
					}
				}
			}
		}
	}
	return p
}
