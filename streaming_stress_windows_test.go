//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
	"unsafe"
)

// Opt-in real TCP/meshing/GPU cold-load probe. Never opens a personal save or
// overwrites performance.log. Limits stop the run, not the production loader.
func TestStreaming128Stress(t *testing.T) {
	if os.Getenv("GOCRAFT_STRESS128") != "1" {
		t.Skip("opt-in 128 radius GPU stress")
	}
	runStreamingProbe(t, 128, 45*time.Second)
}

// Fixed-camera end-to-end profile of the two practical long view distances.
// Kept opt-in because it starts real TCP, chunk workers and a GPU session.
func TestStreamingRadiusProfile(t *testing.T) {
	if os.Getenv("GOCRAFT_STREAM_PROFILE") != "1" {
		t.Skip("opt-in 32/64 radius GPU loading profile")
	}
	for _, radius := range []int{32, 64} {
		t.Run(fmt.Sprintf("radius%d", radius), func(t *testing.T) {
			runStreamingProbe(t, radius, 75*time.Second)
		})
	}
}

func streamingProbeProgress(w *World, plan *chunkRequestPlan) (received, target int) {
	if !plan.valid {
		return
	}
	w.chunksMu.RLock()
	defer w.chunksMu.RUnlock()
	target = len(plan.keys)
	for _, key := range plan.keys {
		c := w.chunks[key]
		if c == nil || !c.generated {
			continue
		}
		received++
	}
	return
}

func runStreamingProbe(t *testing.T, radius int, duration time.Duration) {
	t.Helper()
	r, cleanup := nativePreviewFixture(t)
	defer cleanup()
	var w *nativeWindow
	for _, candidate := range winWindows {
		w = candidate
	}
	if w == nil {
		t.Fatal("no fixture window")
	}
	oldRenderer, oldWindow, oldPM := activeWebGPUWorldRenderer, nativeGameWindow, perfMon
	oldWorld, oldClient, oldServer, oldInput := world, client, server, input
	oldState, oldCamera, oldMode, oldRadius := currentState, camera, currentGameMode, currentSettings.RenderDistance
	defer func() {
		activeWebGPUWorldRenderer, nativeGameWindow, perfMon = oldRenderer, oldWindow, oldPM
		world, client, server, input = oldWorld, oldClient, oldServer, oldInput
		currentState, camera, currentGameMode = oldState, oldCamera, oldMode
		currentSettings.RenderDistance = oldRadius
	}()
	activeWebGPUWorldRenderer, nativeGameWindow = r, w
	currentSettings.RenderDistance = radius
	save := t.TempDir()
	if err := SaveLevelData(save, 12345); err != nil {
		t.Fatal(err)
	}
	perfMon = &PerformanceMonitor{startTime: time.Now()}
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("work/stream-r%d", radius)
	if radius == 128 {
		prefix = "work/stress128"
	}
	log, err := os.Create(prefix + ".csv")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	perfMon.file = log
	_, _ = log.WriteString("Timestamp,FPS,FrameTime(ms),HeapAlloc(MB),Goroutines,MeshesBuilt/s,ClientChunksReceived/s,ChunksUnloaded/s,ActiveMeshes,FrameP95(ms),FrameP99(ms),ServerTick(ms),MeshJobs,MeshResults,ClientChunks,DrawCalls,Triangles" + loadingCSVHeader() + "\n")
	progress, err := os.Create(prefix + "-progress.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer progress.Close()
	_, _ = progress.WriteString("Seconds,Received,Target,VisibleSections,ReadySections,PendingRequests,IncomingQueued,DrawCalls,FrameP95MS,FrameP99MS,HeapMiB\n")
	profile, err := os.Create(prefix + ".cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer profile.Close()
	if err = pprof.StartCPUProfile(profile); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()
	startGame(save, "", false)
	if currentState != StatePlaying {
		t.Fatal("session failed", menuError)
	}
	defer exitGame()
	start, last, nextLog := time.Now(), time.Now(), time.Now()
	playableAt := time.Duration(-1)
	readyAt := time.Duration(-1)
	for time.Since(start) < duration {
		now := time.Now()
		dt := float32(now.Sub(last).Seconds())
		last = now
		w.Poll()
		setGameFrameTime(dt)
		updateGame()
		if client == nil || world == nil {
			t.Fatal("unexpected disconnect")
		}
		if playableAt < 0 && input != nil && input.VitalsReady && world.getChunkIfGenerated(int(camera.Position.X)/chunkWidth, int(camera.Position.Z)/chunkWidth) != nil {
			playableAt = time.Since(start)
		}
		// Fixed aerial camera: isolate loading from player input/death. Server
		// position is synchronized by the normal next-frame movement update.
		currentGameMode = ModeCreative
		camera.Position.Y = 143
		camera.Target = newGameVec3(camera.Position.X+100, 100, camera.Position.Z+100)
		if err := drawExperimentalWebGPUFrame(); err != nil {
			t.Fatal(err)
		}
		perfMon.frames = append(perfMon.frames, float64(dt)*1000)
		perfMon.Metrics.FrameTime = dt * 1000
		if !now.Before(nextLog) {
			perfMon.logMetrics()
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			received, target := streamingProbeProgress(world, &chunkRequests)
			visible, ready := world.render.visibleSections, world.render.readySections
			_, _ = fmt.Fprintf(progress, "%.1f,%d,%d,%d,%d,%d,%d,%d,%.2f,%.2f,%.0f\n", time.Since(start).Seconds(), received, target, visible, ready, len(pendingChunkRequests), len(client.Incoming), world.render.drawCalls, perfMon.Metrics.FrameP95, perfMon.Metrics.FrameP99, float64(mem.HeapAlloc)/(1<<20))
			t.Logf("%.1fs radius=%d received=%d/%d visible-ready=%d/%d draws=%d heap=%.0fMiB queue=%d pending=%d", time.Since(start).Seconds(), radius, received, target, ready, visible, world.render.drawCalls, float64(mem.HeapAlloc)/(1<<20), len(client.Incoming), len(pendingChunkRequests))
			if target > 0 && received == target && ready == visible && len(world.meshJobs) == 0 && len(world.meshResults) == 0 {
				readyAt = time.Since(start)
				break
			}
			nextLog = now.Add(time.Second)
			// 4GiB free-system guard also protects other applications while testing.
			var status struct {
				Length, Load                                                                          uint32
				TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
			}
			status.Length = uint32(unsafe.Sizeof(status))
			ok, _, _ := winKernel.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&status)))
			if mem.HeapAlloc > 6<<30 || (ok != 0 && status.AvailPhys < 4<<30) {
				t.Log("stopped at memory safety budget")
				break
			}
		}
		if rest := time.Second/60 - time.Since(now); rest > 0 {
			time.Sleep(rest)
		}
	}
	perfMon.logMetrics()
	t.Logf("radius %d: playable=%s all-mesh-ready=%s elapsed=%.2fs; shutdown/save excluded from CSV", radius, playableAt, readyAt, time.Since(start).Seconds())
}
