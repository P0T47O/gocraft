//go:build windows

package main

import (
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
	oldState, oldCamera, oldMode := currentState, camera, currentGameMode
	defer func() {
		activeWebGPUWorldRenderer, nativeGameWindow, perfMon = oldRenderer, oldWindow, oldPM
		world, client, server, input = oldWorld, oldClient, oldServer, oldInput
		currentState, camera, currentGameMode = oldState, oldCamera, oldMode
	}()
	activeWebGPUWorldRenderer, nativeGameWindow = r, w
	currentSettings.RenderDistance = 128
	save := t.TempDir()
	if err := SaveLevelData(save, 12345); err != nil {
		t.Fatal(err)
	}
	perfMon = &PerformanceMonitor{startTime: time.Now()}
	log, err := os.Create("work/stress128.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	perfMon.file = log
	_, _ = log.WriteString("Timestamp,FPS,FrameTime(ms),HeapAlloc(MB),Goroutines,MeshesBuilt/s,ClientChunksReceived/s,ChunksUnloaded/s,ActiveMeshes,FrameP95(ms),FrameP99(ms),ServerTick(ms),MeshJobs,MeshResults,ClientChunks,DrawCalls,Triangles" + loadingCSVHeader() + "\n")
	profile, err := os.Create("work/stress128.cpu")
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
	for time.Since(start) < 45*time.Second {
		now := time.Now()
		dt := float32(now.Sub(last).Seconds())
		last = now
		w.Poll()
		setGameFrameTime(dt)
		updateGame()
		if client == nil || world == nil {
			t.Fatal("unexpected disconnect")
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
			t.Logf("%.1fs heap=%.0fMiB received=%d queue=%d pending=%d gen=%d/%d mesh=%d/%d", time.Since(start).Seconds(), float64(mem.HeapAlloc)/(1<<20), len(world.chunks), len(client.Incoming), len(pendingChunkRequests), perfMon.genQueued.Load(), perfMon.genReady.Load(), len(world.meshJobs), len(world.meshResults))
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
	t.Logf("measurement complete in %.2fs; shutdown/save excluded from CSV", time.Since(start).Seconds())
}
