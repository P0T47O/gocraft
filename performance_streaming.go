package main

import "time"

// Main-loop-only timestamps retained across retries, bounded by the requested
// radius and cleared on session changes. Ready means CPU submission completed,
// not GPU completion or a guarantee that the chunk was visible on screen.
var chunkLoadStarts = make(map[chunkKey]time.Time)

func recordChunkReady(key chunkKey, c *Chunk) {
	start, ok := chunkLoadStarts[key]
	if !ok {
		return
	}
	for sec := 0; sec < sectionCount; sec++ {
		if c.sectionBlocks[sec] != 0 && (sec >= len(c.sectionDirty) || c.sectionDirty[sec]) {
			return
		}
	}
	if perfMon != nil {
		perfMon.readyLatency = append(perfMon.readyLatency, float64(time.Since(start))/float64(time.Millisecond))
	}
	delete(chunkLoadStarts, key)
}

func resetChunkStreaming() {
	clear(pendingChunkRequests)
	clear(chunkLoadStarts)
	chunkRequests = chunkRequestPlan{}
	if perfMon != nil {
		perfMon.readyLatency = nil
		perfMon.receiveLatency = nil
	}
}
