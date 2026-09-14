package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
)

func drawGame() {
	camBlockX := int(math.Floor(float64(camera.Position.X) + 0.5))
	camBlockY := int(math.Floor(float64(camera.Position.Y) + 0.5))
	camBlockZ := int(math.Floor(float64(camera.Position.Z) + 0.5))
	inWater := world.BlockAt(camBlockX, camBlockY, camBlockZ) == blockWater

	rl.BeginMode3D(camera)

	// Draw World
	background := rl.NewColor(180, 210, 255, 255)
	if inWater {
		background = rl.NewColor(40, 70, 120, 255)
	}
	rl.ClearBackground(background)

	world.Draw(assets, camera)

	// Draw Entities
	for id, e := range remoteEntities {
		if id == *username {
			continue
		}
		if e.Type == EntityPlayer {
			drawCharacterModel(rl.NewVector3(float32(e.X), float32(e.Y), float32(e.Z)), e.Yaw)
		} else if e.Type == EntityItem {
			assets.DrawItem(e)
		} else if e.MobKind != "" && mobsRenderer != nil {
			mobsRenderer.Draw(e, input.ShowDebug)
		} else {
			pos := rl.NewVector3(float32(e.X), float32(e.Y)+0.5, float32(e.Z))
			rl.DrawCube(pos, 0.8, 0.8, 0.8, rl.Pink)
		}
	}

	rl.EndMode3D()

	// Draw Mining Crack Overlay
	world.DrawBlockCrack(assets, camera, input)

	// 2D Overlay
	if inWater {
		overlay := rl.NewColor(40, 90, 160, 120)
		rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), overlay)
	}

	if !input.InventoryOpen && !isPaused && !input.isDead() {
		assets.drawCrosshair()
	}

	if input.ShowDebug && perfMon != nil {
		m := perfMon.Metrics
		// Background for readability
		rl.DrawRectangle(5, 5, 550, 225, rl.Fade(invBackground, 0.92))

		rl.DrawText(fmt.Sprintf("%d FPS", rl.GetFPS()), 10, 10, 20, invAccent)
		rl.DrawText(fmt.Sprintf("Pos: %.1f, %.1f, %.1f", camera.Position.X, camera.Position.Y, camera.Position.Z), 10, 35, 20, invText)

		rl.DrawText(fmt.Sprintf("Chunks: %d loaded / %d mesh buffers", len(world.chunks), m.ActiveMeshes), 10, 60, 20, invText)
		rl.DrawText(fmt.Sprintf("%d chunk updates/sec", m.MeshesPerSec), 10, 85, 20, invText)
		rl.DrawText(fmt.Sprintf("Mem: %d MB (GC: %d)", m.HeapAllocMB, m.NumGC), 10, 110, 20, invText)
		rl.DrawText(fmt.Sprintf("Unloads/sec: %d", m.UnloadsPerSec), 10, 135, 20, invText)
		rl.DrawText(fmt.Sprintf("Frame p95/p99: %.1f/%.1f ms; Tick: %.1f ms", m.FrameP95, m.FrameP99, m.ServerTickMS), 10, 160, 18, invText)
		rl.DrawText(fmt.Sprintf("Mesh queue: %d/%d; Draws: %d", m.MeshJobs, m.MeshResults, m.DrawCalls), 10, 183, 18, invText)
		rl.DrawText(fmt.Sprintf("Triangles: %d", m.Triangles), 10, 206, 18, invText)
	}

	if input.InventoryOpen {
		assets.drawInventory(input)
	} else {
		assets.drawHotbar(input)
	}

	if !input.InventoryOpen {
		drawVitalsHUD(input)
	}
	if input.isDead() {
		drawDeathScreen(input)
		return
	}
	drawChatOverlay()

	if isPaused {
		drawPauseMenu()
	}
}

func drawCharacterModel(pos rl.Vector3, yaw float32) {
	rl.PushMatrix()
	rl.Translatef(pos.X, pos.Y, pos.Z)
	rl.Rotatef(-yaw, 0, 1, 0)

	// Body
	rl.DrawCube(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.DarkGray)
	rl.DrawCubeWires(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.Black)

	// Head
	rl.DrawCube(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.LightGray)
	rl.DrawCubeWires(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.Black)

	rl.PopMatrix()
}
