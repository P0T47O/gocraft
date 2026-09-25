package main

const sleepStartTick = 12500

// A bed is currently a single low block, so sleeping is represented as a
// server-side vote rather than moving the player into an unimplemented pose.
func (s *Server) useBed(p *PlayerEntity, pos BlockPos) bool {
	if p == nil || p.dead() || p.Vitals == nil || s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z)) != blockBed {
		return false
	}
	p.Vitals.HomeX, p.Vitals.HomeY, p.Vitals.HomeZ, p.Vitals.HasHome = pos.X, pos.Y, pos.Z, true
	s.SendTo(p.UUID, &PacketChat{Message: "Respawn point set."})
	phase := int64(s.World.TimeTicks) % 24000
	if phase >= sleepStartTick && phase < 23000 {
		if s.sleeping == nil {
			s.sleeping = make(map[string]BlockPos)
		}
		s.sleeping[p.UUID] = pos
		if s.allSurvivalPlayersSleeping() {
			s.World.TimeTicks += float64(24000 - phase)
			s.sleeping = nil
			s.Broadcast(&PacketWorldTime{Ticks: int64(s.World.TimeTicks)})
		} else {
			s.SendTo(p.UUID, &PacketChat{Message: "Waiting for other players to sleep."})
		}
	}
	return true
}

func (s *Server) allSurvivalPlayersSleeping() bool {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	for name := range s.Clients {
		p := s.findPlayerEntity(name)
		if p == nil || p.dead() || p.GameMode != ModeSurvival {
			continue
		}
		pos, waiting := s.sleeping[name]
		if !waiting || s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z)) != blockBed || !s.containerReach(p, pos) {
			return false
		}
	}
	return true
}
