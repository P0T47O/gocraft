package main

import (
	"fmt"
	"gocraft/platform"
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
	return s + ",GenerationQueued,GenerationReady,ChunksPending,ChunksPublished,MeshStale,ClientPacketsQueued,RequestReceiveCount,RequestReceiveP95MS,RequestReceiveP99MS,RequestReadyCount,RequestReadyP95MS,RequestReadyP99MS,ClientRequestsPending,ClientReadyPending,WorldMeshBufferBytes,WorldMeshUploadedBytes"
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
	r95, r99 := framePercentiles(pm.receiveLatency)
	m95, m99 := framePercentiles(pm.readyLatency)
	extra := fmt.Sprintf(",%d,%.3f,%.3f,%d,%.3f,%.3f,%d,%d", len(pm.receiveLatency), r95, r99, len(pm.readyLatency), m95, m99, len(pendingChunkRequests), len(chunkLoadStarts))
	pm.receiveLatency = pm.receiveLatency[:0]
	pm.readyLatency = pm.readyLatency[:0]
	extra += fmt.Sprintf(",%d,%d", platform.MeshBufferBytes.Load(), platform.MeshUploadedBytes.Swap(0))
	return s + fmt.Sprintf(",%d,%d,%d,%d,%d,%d", pm.genQueued.Load(), pm.genReady.Load(), pm.chunksPending.Load(), pm.chunksPublished.Swap(0), pm.meshStale.Swap(0), queued) + extra
}
