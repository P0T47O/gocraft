package main

func (s *InputState) isDead() bool { return s.VitalsReady && s.Vitals.Health <= 0 }
func (s *InputState) updateRespawnRequest(c *Client) {
	now := inputTime()
	if s.RespawnWaiting && c != nil && (s.RespawnLast == 0 || now-s.RespawnLast > 1) {
		c.Send(&PacketRespawn{})
		s.RespawnLast = now
	}
}
