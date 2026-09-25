package main

func (s *Server) tickWorldTime() {
	s.World.TimeTicks++
	if int64(s.World.TimeTicks)%timeSyncInterval == 0 {
		s.Broadcast(&PacketWorldTime{Ticks: int64(s.World.TimeTicks)})
	}
}
