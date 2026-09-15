package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"time"

	"gocraft/platform"
	"sync/atomic"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type PerformanceMonitor struct {
	loading                                                        [loadingPhaseCount]loadingPhaseCounter
	genQueued, genReady, chunksPending, chunksPublished, meshStale atomic.Int64
	file                                                           *os.File
	updateTicker                                                   *time.Ticker
	Metrics                                                        PerfMetrics
	chunksMeshed                                                   atomic.Int64
	chunksLoaded                                                   atomic.Int64
	chunksUnloaded                                                 atomic.Int64
	tickNanos                                                      atomic.Int64
	frames                                                         []float64
	startTime                                                      time.Time
}

type PerfMetrics struct {
	FPS                                                       int32
	FrameTime                                                 float32
	HeapAllocMB                                               uint64
	NumGC                                                     uint32
	Goroutines                                                int
	MeshesPerSec                                              int
	ChunksPerSec                                              int
	UnloadsPerSec                                             int
	ActiveMeshes                                              int64
	FrameP95, FrameP99, ServerTickMS                          float64
	MeshJobs, MeshResults, LoadedChunks, DrawCalls, Triangles int
}

func NewPerformanceMonitor() *PerformanceMonitor {
	f, err := os.Create("performance.log")
	if err != nil {
		log.Printf("Failed to create performance log: %v", err)
		return nil
	}

	// Write CSV Header
	_, _ = f.WriteString("Timestamp,FPS,FrameTime(ms),HeapAlloc(MB),Goroutines,MeshesBuilt/s,ClientChunksReceived/s,ChunksUnloaded/s,ActiveMeshes,FrameP95(ms),FrameP99(ms),ServerTick(ms),MeshJobs,MeshResults,ClientChunks,DrawCalls,Triangles" + loadingCSVHeader() + "\n")

	pm := &PerformanceMonitor{
		file:         f,
		updateTicker: time.NewTicker(1 * time.Second), // Log every second
		startTime:    time.Now(),
		frames:       make([]float64, 0, 512),
	}

	return pm
}

func (pm *PerformanceMonitor) Close() {
	if pm == nil {
		return
	}
	if pm.file != nil {
		pm.file.Close()
	}
	pm.updateTicker.Stop()
}

func (pm *PerformanceMonitor) IncrementMeshBuild() {
	if pm == nil {
		return
	}
	pm.chunksMeshed.Add(1)
}

func (pm *PerformanceMonitor) IncrementChunkLoad() {
	if pm == nil {
		return
	}
	pm.chunksLoaded.Add(1)
}

func (pm *PerformanceMonitor) IncrementChunkUnload() {
	if pm == nil {
		return
	}
	pm.chunksUnloaded.Add(1)
}

func (pm *PerformanceMonitor) RecordTick(elapsed time.Duration) {
	if pm != nil {
		pm.tickNanos.Store(int64(elapsed))
	}
}

func framePercentiles(samples []float64) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	sort.Float64s(samples)
	return samples[(len(samples)*95+99)/100-1], samples[(len(samples)*99+99)/100-1]
}

func performanceFrameTime() float32 {
	if currentState == StatePlaying {
		return gameFrameTime()
	}
	return rl.GetFrameTime()
}

func (pm *PerformanceMonitor) Update() {
	if pm == nil || pm.file == nil {
		return
	}
	if rl.IsWindowReady() {
		pm.frames = append(pm.frames, float64(performanceFrameTime())*1000)
	}

	select {
	case <-pm.updateTicker.C:
		pm.logMetrics()
	default:
		// Do nothing
	}
}

func (pm *PerformanceMonitor) logMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if rl.IsWindowReady() {
		pm.Metrics.FrameTime = performanceFrameTime() * 1000.0 // ms
	}
	var totalMS float64
	for _, frame := range pm.frames {
		totalMS += frame
	}
	if totalMS > 0 {
		pm.Metrics.FPS = int32(float64(len(pm.frames))*1000/totalMS + 0.5)
	}
	pm.Metrics.HeapAllocMB = m.HeapAlloc / 1024 / 1024
	pm.Metrics.NumGC = m.NumGC
	pm.Metrics.Goroutines = runtime.NumGoroutine()
	pm.Metrics.MeshesPerSec = int(pm.chunksMeshed.Swap(0))
	pm.Metrics.ChunksPerSec = int(pm.chunksLoaded.Swap(0))
	pm.Metrics.UnloadsPerSec = int(pm.chunksUnloaded.Swap(0))
	pm.Metrics.ActiveMeshes = atomic.LoadInt64(&platform.ActiveMeshCount)

	pm.Metrics.FrameP95, pm.Metrics.FrameP99 = framePercentiles(pm.frames)
	pm.frames = pm.frames[:0]
	pm.Metrics.ServerTickMS = float64(pm.tickNanos.Load()) / float64(time.Millisecond)
	if world != nil {
		pm.Metrics.MeshJobs = len(world.meshJobs)
		pm.Metrics.MeshResults = len(world.meshResults)
		pm.Metrics.LoadedChunks = len(world.chunks)
		pm.Metrics.DrawCalls = world.render.drawCalls
		pm.Metrics.Triangles = world.render.triangles
	} else {
		pm.Metrics.MeshJobs = 0
		pm.Metrics.MeshResults = 0
		pm.Metrics.LoadedChunks = 0
		pm.Metrics.DrawCalls = 0
		pm.Metrics.Triangles = 0
		pm.Metrics.ServerTickMS = 0
	}

	timestamp := time.Since(pm.startTime).Seconds()

	line := fmt.Sprintf("%.2f,%d,%.2f,%d,%d,%d,%d,%d,%d,%.2f,%.2f,%.2f,%d,%d,%d,%d,%d",
		timestamp,
		pm.Metrics.FPS,
		pm.Metrics.FrameTime,
		pm.Metrics.HeapAllocMB,
		pm.Metrics.Goroutines,
		pm.Metrics.MeshesPerSec,
		pm.Metrics.ChunksPerSec,
		pm.Metrics.UnloadsPerSec,
		pm.Metrics.ActiveMeshes,
		pm.Metrics.FrameP95, pm.Metrics.FrameP99, pm.Metrics.ServerTickMS,
		pm.Metrics.MeshJobs, pm.Metrics.MeshResults, pm.Metrics.LoadedChunks, pm.Metrics.DrawCalls, pm.Metrics.Triangles,
	)

	_, err := pm.file.WriteString(line + pm.loadingCSVValues() + "\n")
	if err != nil {
		log.Printf("Error writing to perf log: %v", err)
	}
}
