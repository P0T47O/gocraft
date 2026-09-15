package main

import "time"

// Independent from gameplay TPS, but still executed by the single world owner.
// Each phase has a time budget. Backlog expands batches, not unbounded work.
func (s *Server) processChunkStreaming() {
	if s.Paused.Load() {
		return
	}
	s.World.ProcessGenResults()
	s.processPendingChunks()
	if perfMon != nil {
		perfMon.genQueued.Store(int64(len(s.World.genQueue)))
		perfMon.genReady.Store(int64(len(s.World.genResults)))
		perfMon.chunksPending.Store(int64(len(s.PendingChunks)))
	}
}

func streamingBatchSize(backlog int) int { return min(32, max(4, backlog)) }

type chunkPriority struct {
	key      chunkKey
	distance int
}

// Refresh at most once per 50ms, amortizing sorting while preserving near-first
// loading after movement. New requests enter on the next refresh (at most 50ms).
func (s *Server) chunkOrderNeedsRefresh(now time.Time) bool {
	return len(s.chunkOrder) == 0 || now.Sub(s.chunkOrderAt) >= 50*time.Millisecond
}

// Leave room for vital gameplay packets; a slow peer cannot block other peers.
func chunkSendHasRoom(c *ClientConnection) bool {
	return c != nil && len(c.Send) < max(1, cap(c.Send)*3/4)
}
