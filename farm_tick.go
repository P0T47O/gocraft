package main

import (
	"math"
	"math/rand"
)

// Random columns near online players bound farm work independently of view
// distance. Crop stage lives in saved block metadata, so reloaded fields keep
// growing without a separate in-memory crop registry.
func (s *Server) tickFarms() {
	s.World.entitiesMu.RLock()
	players := make([][2]int, 0, 4)
	for _, e := range s.World.entities {
		p, ok := e.(*PlayerEntity)
		if !ok || p.dead() {
			continue
		}
		s.ClientsMu.RLock()
		online := s.Clients[p.UUID] != nil
		s.ClientsMu.RUnlock()
		if online {
			players = append(players, [2]int{int(math.Floor(p.X)), int(math.Floor(p.Z))})
		}
	}
	s.World.entitiesMu.RUnlock()
	for _, center := range players {
		for i := 0; i < 24; i++ {
			x, z := center[0]+rand.Intn(33)-16, center[1]+rand.Intn(33)-16
			s.sampleFarmColumn(x, z)
		}
	}
}

func (s *Server) sampleFarmColumn(x, z int) {
	chunk := s.World.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth))
	if chunk == nil {
		return
	}
	// Scan under one chunk lock. Crops under a roof or in a lit underground
	// farm grow as well; the work is capped at 24 sampled columns per player.
	lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
	var small [8]int
	cropYs := small[:0]
	chunk.mu.RLock()
	for y := min(chunkHeight-1, int(chunk.heightMap[lx][lz])-1); y > 0; y-- {
		if isCrop(chunk.blocks.Get(lx, y, lz)) {
			cropYs = append(cropYs, y)
		}
	}
	chunk.mu.RUnlock()
	for _, y := range cropYs {
		s.growCropAt(x, y, z)
	}
}

func (s *Server) growCropAt(x, y, z int) {
	crop := s.World.BlockAt(x, y, z)
	if !isCrop(crop) {
		return
	}
	if s.World.BlockAt(x, y-1, z) != blockFarmland {
		s.dropCrop(x, y, z, crop, s.World.MetaAt(x, y, z))
		s.setFarmBlock(x, y, z, blockAir, 0)
		return
	}
	stage := min(s.World.MetaAt(x, y, z), cropMaxStage(crop))
	if stage >= cropMaxStage(crop) {
		return
	}
	if max(s.World.LightSkyAt(x, y, z), s.World.LightBlockAt(x, y, z)) < 9 {
		return
	}
	moist := byte(0)
	if s.nearbyFarmWater(x, y-1, z) {
		moist = 1
	}
	if s.World.MetaAt(x, y-1, z) != moist {
		s.setFarmBlock(x, y-1, z, blockFarmland, moist)
	}
	chance := 4
	if moist == 0 {
		chance = 12
	}
	if rand.Intn(chance) == 0 {
		s.setFarmBlock(x, y, z, crop, stage+1)
	}
}
