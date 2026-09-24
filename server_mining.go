package main

import (
	"math"
	"time"
)

func (s *Server) blockChangeReach(player *PlayerEntity, pos BlockPos) bool {
	if player == nil || player.dead() {
		return false
	}
	if player.GameMode != ModeCreative {
		return s.containerReach(player, pos)
	}
	// Creative's client ray reaches 8 blocks; placing on the far face adds a
	// block, and this check measures to the block corner rather than the hit.
	dx, dy, dz := player.X-float64(pos.X), player.Y-float64(pos.Y), player.Z-float64(pos.Z)
	return !math.IsNaN(dx+dy+dz) && dx*dx+dy*dy+dz*dz <= 121
}

// A start request does not change the world. Only the server clock and the
// server's current block/tool state can authorize the later break request.
type miningSession struct {
	pos     BlockPos
	block   byte
	tool    byte
	slot    int
	started time.Time
}

func heldMiningTool(player *PlayerEntity) byte {
	if player.SelectedSlot < 0 || player.SelectedSlot >= 9 {
		return 0
	}
	stack := player.Inventory.Slots[player.SelectedSlot]
	if stack.Count <= 0 || stack.ID <= 0 || stack.ID > 255 {
		return 0
	}
	return byte(stack.ID)
}

func (s *Server) beginMining(player *PlayerEntity, pos BlockPos, now time.Time) {
	delete(s.miningSessions, player.UUID)
	if player.GameMode != ModeSurvival || pos.Y < 0 || pos.Y >= chunkHeight || !s.containerReach(player, pos) {
		return
	}
	block := s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z))
	if block == blockAir || MiningSeconds(heldMiningTool(player), block) < 0 {
		return
	}
	if s.miningSessions == nil {
		s.miningSessions = make(map[string]miningSession)
	}
	s.miningSessions[player.UUID] = miningSession{pos: pos, block: block, tool: heldMiningTool(player), slot: player.SelectedSlot, started: now}
}

func (s *Server) mayFinishMining(player *PlayerEntity, pos BlockPos, block byte, now time.Time) bool {
	session, ok := s.miningSessions[player.UUID]
	delete(s.miningSessions, player.UUID) // One start authorizes at most one break.
	if !ok || session.pos != pos || session.block != block || session.slot != player.SelectedSlot ||
		session.tool != heldMiningTool(player) || !s.containerReach(player, pos) {
		return false
	}
	required := float64(MiningSeconds(session.tool, block))
	if required < 0 {
		return false
	}
	// Permit only a small clock/packet scheduling margin, not client-reported progress.
	grace := min(0.05, required*0.1)
	return now.Sub(session.started).Seconds() >= required-grace
}
