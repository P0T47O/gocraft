package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"gocraft/platform"
	"sync/atomic"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type PerformanceMonitor struct {
	file           *os.File
	updateTicker   *time.Ticker
	Metrics        PerfMetrics
	chunksMeshed   int
	chunksLoaded   int
	chunksUnloaded int
	startTime      time.Time
}

type PerfMetrics struct {
	FPS           int32
	FrameTime     float32
	HeapAllocMB   uint64
	NumGC         uint32
	Goroutines    int
	MeshesPerSec  int
	ChunksPerSec  int
	UnloadsPerSec int
	ActiveMeshes  int64
}

func NewPerformanceMonitor() *PerformanceMonitor {
	f, err := os.Create("performance.log")
	if err != nil {
		log.Printf("Failed to create performance log: %v", err)
		return nil
	}

	// Write CSV Header
	_, _ = f.WriteString("Timestamp,FPS,FrameTime(ms),HeapAlloc(MB),Goroutines,MeshesBuilt/s,ChunksLoaded/s,ChunksUnloaded/s,ActiveMeshes\n")

	pm := &PerformanceMonitor{
		file:         f,
		updateTicker: time.NewTicker(1 * time.Second), // Log every second
		startTime:    time.Now(),
	}

	return pm
}

func (pm *PerformanceMonitor) Close() {
	if pm.file != nil {
		pm.file.Close()
	}
	pm.updateTicker.Stop()
}

func (pm *PerformanceMonitor) IncrementMeshBuild() {
	if pm == nil {
		return
	}
	pm.chunksMeshed++
}

func (pm *PerformanceMonitor) IncrementChunkLoad() {
	if pm == nil {
		return
	}
	pm.chunksLoaded++
}

func (pm *PerformanceMonitor) IncrementChunkUnload() {
	if pm == nil {
		return
	}
	pm.chunksUnloaded++
}

func (pm *PerformanceMonitor) Update() {
	if pm == nil || pm.file == nil {
		return
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
		pm.Metrics.FPS = rl.GetFPS()
		pm.Metrics.FrameTime = rl.GetFrameTime() * 1000.0 // ms
	}
	pm.Metrics.HeapAllocMB = m.HeapAlloc / 1024 / 1024
	pm.Metrics.NumGC = m.NumGC
	pm.Metrics.Goroutines = runtime.NumGoroutine()
	pm.Metrics.MeshesPerSec = pm.chunksMeshed
	pm.Metrics.ChunksPerSec = pm.chunksLoaded
	pm.Metrics.UnloadsPerSec = pm.chunksUnloaded
	pm.Metrics.ActiveMeshes = atomic.LoadInt64(&platform.ActiveMeshCount)

	// Reset counters
	pm.chunksMeshed = 0
	pm.chunksLoaded = 0
	pm.chunksUnloaded = 0

	timestamp := time.Since(pm.startTime).Seconds()

	line := fmt.Sprintf("%.2f,%d,%.2f,%d,%d,%d,%d,%d,%d\n",
		timestamp,
		pm.Metrics.FPS,
		pm.Metrics.FrameTime,
		pm.Metrics.HeapAllocMB,
		pm.Metrics.Goroutines,
		pm.Metrics.MeshesPerSec,
		pm.Metrics.ChunksPerSec,
		pm.Metrics.UnloadsPerSec,
		pm.Metrics.ActiveMeshes,
	)

	_, err := pm.file.WriteString(line)
	if err != nil {
		log.Printf("Error writing to perf log: %v", err)
	}
}
