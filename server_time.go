package main

func (s *Server) tickWorldTime() {
	s.World.TimeTicks++
	phase := int64(s.World.TimeTicks) % 24000
	if phase < sleepStartTick || phase >= 23000 {
		s.sleeping = nil
	}
	if int64(s.World.TimeTicks)%timeSyncInterval == 0 {
		s.Broadcast(&PacketWorldTime{Ticks: int64(s.World.TimeTicks)})
	}
}
