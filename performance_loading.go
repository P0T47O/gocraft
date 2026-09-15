package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

type loadingPhase int

const (
	phaseDisk loadingPhase = iota
	phaseGeneration
	phaseLightInit
	phaseLightStitch
	phaseSnapshot
	phaseNetworkWrite
	phaseClientApply
	phaseMeshCPU
	phaseUpload
	loadingPhaseCount
)

var loadingPhaseNames = [...]string{"DiskLoad", "Generation", "LightInit", "LightStitch", "Snapshot", "NetworkWrite", "ClientApply", "MeshCPU", "Upload"}

type loadingPhaseCounter struct{ nanos, calls atomic.Int64 }

func (pm *PerformanceMonitor) recordLoading(phase loadingPhase, start time.Time) {
	if pm == nil {
		return
	}
	pm.loading[phase].nanos.Add(int64(time.Since(start)))
	pm.loading[phase].calls.Add(1)
}

func loadingCSVHeader() string {
	s := ""
	for _, name := range loadingPhaseNames {
		s += "," + name + "WorkMS," + name + "Calls"
	}
	return s + ",GenerationQueued,GenerationReady,ChunksPending,ChunksPublished,MeshStale,ClientPacketsQueued"
}

// WorkMS is aggregate wall time across workers per logging interval, not frame
// latency. NetworkWrite includes encoding and socket backpressure, not RTT.
func (pm *PerformanceMonitor) loadingCSVValues() string {
	s := ""
	for i := range pm.loading {
		s += fmt.Sprintf(",%.3f,%d", float64(pm.loading[i].nanos.Swap(0))/float64(time.Millisecond), pm.loading[i].calls.Swap(0))
	}
	queued := 0
	if client != nil {
		queued = len(client.Incoming)
	}
	return s + fmt.Sprintf(",%d,%d,%d,%d,%d,%d", pm.genQueued.Load(), pm.genReady.Load(), pm.chunksPending.Load(), pm.chunksPublished.Swap(0), pm.meshStale.Swap(0), queued)
}
