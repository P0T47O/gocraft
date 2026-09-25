package main

const (
	fluidMaxQueued         = 65536
	fluidMaxVisitsPerTick  = 128
	fluidMaxChangesPerTick = 64
	fluidFalling           = 8
)

type fluidTask struct {
	pos BlockPos
	due uint64
}

var fluidNeighbors = [...]BlockPos{{0, -1, 0}, {0, 1, 0}, {-1, 0, 0}, {1, 0, 0}, {0, 0, -1}, {0, 0, 1}}
var fluidHorizontals = [...]BlockPos{{-1, 0, 0}, {1, 0, 0}, {0, 0, -1}, {0, 0, 1}}

func isFluid(block byte) bool { return block == blockWater || block == blockLava }
func fluidLimit(block byte) byte {
	if block == blockLava {
		return 4
	}
	return 7
}
func fluidDelay(block byte) uint64 {
	if block == blockLava {
		return 4
	}
	return 1
}

// Source and falling fluid occupy a full voxel. Spreading levels form a
// stepped surface; a same-fluid neighbor with a lower surface gets a lip face.
func fluidSurfaceHeight(meta byte) float32 {
	if meta == 0 || meta >= fluidFalling {
		return 1
	}
	return 1 - float32(meta)*0.1
}

func (s *Server) fluidLoaded(pos BlockPos) bool {
	return pos.Y >= 0 && pos.Y < chunkHeight && s.World.getChunkIfGenerated(divFloor(int(pos.X), chunkWidth), divFloor(int(pos.Z), chunkWidth)) != nil
}

func (s *Server) scheduleFluid(pos BlockPos) {
	if !s.fluidLoaded(pos) || !isFluid(s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z))) {
		return
	}
	if s.fluidPending == nil {
		s.fluidPending = make(map[BlockPos]struct{})
	}
	if _, exists := s.fluidPending[pos]; exists {
		return
	}
	if len(s.fluidQueue)-s.fluidHead >= fluidMaxQueued {
		return
	}
	block := s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z))
	s.fluidPending[pos] = struct{}{}
	s.fluidQueue = append(s.fluidQueue, fluidTask{pos, s.fluidClock + fluidDelay(block)})
}

func (s *Server) scheduleFluidAround(pos BlockPos) {
	s.scheduleFluid(pos)
	for _, d := range fluidNeighbors {
		s.scheduleFluid(BlockPos{pos.X + d.X, pos.Y + d.Y, pos.Z + d.Z})
	}
}

func (s *Server) setFluidBlock(pos BlockPos, block, meta byte) bool {
	if !s.fluidLoaded(pos) || len(s.fluidChanges) >= fluidMaxChangesPerTick {
		return false
	}
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	if s.World.BlockAt(x, y, z) == block && s.World.MetaAt(x, y, z) == meta {
		return true
	}
	s.World.SetBlockAt(x, y, z, block)
	if meta != 0 || s.World.MetaAt(x, y, z) != 0 {
		s.World.SetMetaAt(x, y, z, meta)
	}
	s.fluidChanges = append(s.fluidChanges, FluidChange{pos.X, pos.Y, pos.Z, block, meta})
	s.scheduleFluidAround(pos)
	return true
}

func (s *Server) fluidDesired(pos BlockPos, block, below byte) byte {
	above := BlockPos{pos.X, pos.Y + 1, pos.Z}
	if s.fluidLoaded(above) && s.World.BlockAt(int(above.X), int(above.Y), int(above.Z)) == block {
		if below == block || below == blockAir {
			return fluidFalling
		}
		return 1
	}
	best := fluidLimit(block)
	for _, d := range fluidHorizontals {
		n := BlockPos{pos.X + d.X, pos.Y, pos.Z + d.Z}
		if !s.fluidLoaded(n) || s.World.BlockAt(int(n.X), int(n.Y), int(n.Z)) != block {
			continue
		}
		level := s.World.MetaAt(int(n.X), int(n.Y), int(n.Z))
		if level < best {
			best = level
		}
	}
	if best >= fluidLimit(block) {
		return 255
	}
	return best + 1
}

func (s *Server) fluidStep(pos BlockPos) {
	if !s.fluidLoaded(pos) {
		return
	}
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	block := s.World.BlockAt(x, y, z)
	if !isFluid(block) {
		return
	}
	meta := s.World.MetaAt(x, y, z)
	// A water/lava boundary solidifies lava, never water. Source lava becomes
	// obsidian; flowing lava becomes cobblestone.
	for _, d := range fluidNeighbors {
		n := BlockPos{pos.X + d.X, pos.Y + d.Y, pos.Z + d.Z}
		if !s.fluidLoaded(n) {
			continue
		}
		other := s.World.BlockAt(int(n.X), int(n.Y), int(n.Z))
		if block == blockLava && other == blockWater {
			stone := blockCobblestone
			if meta == 0 {
				stone = blockObsidian
			}
			s.setFluidBlock(pos, stone, 0)
			return
		}
		if block == blockWater && other == blockLava {
			stone := blockCobblestone
			if s.World.MetaAt(int(n.X), int(n.Y), int(n.Z)) == 0 {
				stone = blockObsidian
			}
			if !s.setFluidBlock(n, stone, 0) {
				s.scheduleFluid(pos)
				return
			}
		}
	}
	belowPos := BlockPos{pos.X, pos.Y - 1, pos.Z}
	below := byte(255)
	if s.fluidLoaded(belowPos) {
		below = s.World.BlockAt(x, y-1, z)
	}
	if meta != 0 {
		desired := s.fluidDesired(pos, block, below)
		if desired == 255 {
			s.setFluidBlock(pos, blockAir, 0)
			return
		}
		if desired != meta {
			if !s.setFluidBlock(pos, block, desired) {
				s.scheduleFluid(pos)
				return
			}
			meta = desired
		}
	}
	if below == blockAir || below == block && s.World.MetaAt(x, y-1, z) != 0 && s.World.MetaAt(x, y-1, z) != fluidFalling {
		if !s.setFluidBlock(belowPos, block, fluidFalling) {
			s.scheduleFluid(pos)
			return
		}
	}
	if below == blockAir || meta == fluidFalling {
		return
	}
	next := meta + 1
	if meta == 0 {
		next = 1
	}
	if next > fluidLimit(block) {
		return
	}
	for _, d := range fluidHorizontals {
		n := BlockPos{pos.X + d.X, pos.Y, pos.Z + d.Z}
		if !s.fluidLoaded(n) {
			continue
		}
		other := s.World.BlockAt(int(n.X), int(n.Y), int(n.Z))
		if other == blockAir || other == block && s.World.MetaAt(int(n.X), int(n.Y), int(n.Z)) > next && s.World.MetaAt(int(n.X), int(n.Y), int(n.Z)) != fluidFalling {
			if !s.setFluidBlock(n, block, next) {
				s.scheduleFluid(pos)
				return
			}
		}
	}
}

func (s *Server) tickFluids() {
	s.fluidClock++
	s.fluidChanges = s.fluidChanges[:0]
	visits := min(fluidMaxVisitsPerTick, len(s.fluidQueue)-s.fluidHead)
	for i := 0; i < visits && len(s.fluidChanges) < fluidMaxChangesPerTick; i++ {
		task := s.fluidQueue[s.fluidHead]
		s.fluidHead++
		if task.due > s.fluidClock {
			s.fluidQueue = append(s.fluidQueue, task)
			continue
		}
		delete(s.fluidPending, task.pos)
		s.fluidStep(task.pos)
	}
	if s.fluidHead > 4096 && s.fluidHead*2 >= len(s.fluidQueue) {
		copy(s.fluidQueue, s.fluidQueue[s.fluidHead:])
		s.fluidQueue = s.fluidQueue[:len(s.fluidQueue)-s.fluidHead]
		s.fluidHead = 0
	}
	if len(s.fluidChanges) != 0 {
		packet := &PacketFluidDelta{Changes: append([]FluidChange(nil), s.fluidChanges...)}
		s.Broadcast(packet)
	}
}
