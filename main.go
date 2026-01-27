package main

import (
	"flag"
	"fmt"
	"gocraft/platform"
	"math"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	chunkPool      *ChunkPool
	perfMon        *PerformanceMonitor
	remoteEntities = make(map[string]*RemoteEntity)

	// CLI Flags
	isServer   = flag.Bool("server", false, "Start as dedicated server")
	serverAddr = flag.String("addr", "127.0.0.1:25565", "Server address to listen/connect")
	username   = flag.String("name", "Player"+fmt.Sprint(time.Now().Unix()%1000), "Username")
)

type RemoteEntity struct {
	ID         string
	Type       EntityType
	X, Y, Z    float64 // Visual position (Lerped)
	TX, TY, TZ float64 // Target position (Network)
	Yaw, Pitch float32
}

func main() {
	flag.Parse()

	if *isServer {
		server := NewServer()
		if err := server.StartTCP(*serverAddr); err != nil {
			fmt.Printf("Failed to start server: %v\n", err)
		}
		return
	}
	rl.InitWindow(1920, 1080, "GoCraft")
	defer rl.CloseWindow()

	// Initialize PureGL loader (loads OpenGL function pointers)
	platform.InitGLOnce()

	// rl.SetTargetFPS(60) // Uncap FPS to maximize performance
	rl.SetTargetFPS(0) // 0 usually means unlimited or VSync depending on config
	rl.DisableCursor()

	chunkPool = NewChunkPool(1024)

	initBlockRegistry()

	perfMon = NewPerformanceMonitor()
	defer perfMon.Close()

	assets := loadRenderAssets()
	defer assets.unload()

	camera := rl.Camera3D{
		Position:   rl.NewVector3(8, 8, 20),
		Target:     rl.NewVector3(8, 2, 8),
		Up:         rl.NewVector3(0, 1, 0),
		Fovy:       70,
		Projection: rl.CameraPerspective,
	}

	// --- Client-Server Setup ---
	var server *Server
	client, err := ConnectTCP(*serverAddr, *username)
	if err != nil {
		fmt.Println("No server found, starting internal server...")
		server = NewServer()
		go server.StartTCP(*serverAddr)

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		client, err = ConnectTCP(*serverAddr, *username)
		if err != nil {
			panic("Failed to connect to internal server: " + err.Error())
		}
	}

	// Client World: Background meshing enabled for non-blocking rendering
	world := NewClientWorld()
	world.StartMeshWorkers(assets, 8)

	input := NewInputState()
	// Only load player state for client (Pos, Hotbar).
	// Seed and world chunks will be synced from server later.
	root := filepath.Join(saveDir)
	_ = loadPlayerFile(root, input, &camera, world)
	input.InitFromCamera(camera)

	camera.Target = rl.NewVector3(camera.Position.X, camera.Position.Y-6, camera.Position.Z-12)
	input.InitFromCamera(camera)

	autosaveInterval := float32(45.0)
	autosaveTimer := float32(0.0)

	// Chunk unload timer
	unloadTimer := float32(0.0)
	unloadInterval := float32(1.0)
	renderRadius := 16
	unloadRadius := renderRadius + 4

	for {
		if !input.InventoryOpen && rl.WindowShouldClose() {
			break
		}
		dt := rl.GetFrameTime()
		perfMon.Update()
		assets.Update(dt)

		// 1. Process Network Packets
	PacketLoop:
		for {
			select {
			case pkt := <-client.Incoming:
				switch p := pkt.(type) {
				case *PacketChunkData:
					cx, cz := int(p.CX), int(p.CZ)
					// fmt.Printf("Client: RX Chunk %d,%d, Len %d\n", cx, cz, len(p.Data))
					chunk := world.requestChunk(cx, cz)
					// Manually inject data (Client Mode)
					chunk.mu.Lock()
					idx := 0
					for lx := 0; lx < chunkWidth; lx++ {
						for y := 0; y < chunkHeight; y++ {
							for lz := 0; lz < chunkWidth; lz++ {
								chunk.blocks[lx][y][lz] = p.Data[idx]
								// Unpack light data
								l := p.LightData[idx]
								chunk.skyLight[lx][y][lz] = l >> 4
								chunk.blockLight[lx][y][lz] = l & 0x0F
								idx++
							}
						}
					}
					chunk.rebuildHeightMap()
					chunk.rebuildTorchCount()
					chunk.generated = true
					chunk.dirty = true
					ensureChunkSections(chunk)
					for i := range chunk.sectionDirty {
						chunk.sectionDirty[i] = true
						chunk.meshVersion[i]++
					}
					chunk.mu.Unlock()

					world.markNeighborsDirty(cx, cz)

					// world.rebuildLightingForChunk(cx, cz) // Removed: Light is now sent via PacketChunkData

				case *PacketBlockChange:
					world.SetBlockAt(int(p.X), int(p.Y), int(p.Z), p.BlockID)

				case *PacketPlayerMove:
					// Ignored for now (self-movement is client side prediction)
				case *PacketLogin:
					// Handshake return
					world.seed = p.Seed
					fmt.Printf("Synced with server seed: %d\n", p.Seed)
				case *PacketSpawnPoint:
					camera.Position = rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
					camera.Target = rl.NewVector3(camera.Position.X, camera.Position.Y-2, camera.Position.Z+5)
					input.InitFromCamera(camera)
				case *PacketEntitySpawn:
					remoteEntities[p.EntityID] = &RemoteEntity{
						ID: p.EntityID,
						X:  p.X, Y: p.Y, Z: p.Z,
						TX: p.X, TY: p.Y, TZ: p.Z,
						Yaw: p.Yaw, Pitch: p.Pitch,
					}
				case *PacketEntityDespawn:
					delete(remoteEntities, p.EntityID)
				case *PacketEntityMove:
					if e, ok := remoteEntities[p.EntityID]; ok {
						e.TX, e.TY, e.TZ = p.X, p.Y, p.Z
						e.Yaw, e.Pitch = p.Yaw, p.Pitch
					}
				}
			default:
				break PacketLoop
			}
		}

		world.ProcessMeshResults(assets, 64)

		// 2. Input & Logic
		hit := HandleInput(world, &camera, input, client)

		// Process any immediate meshes (e.g. from interaction) if needed
		world.ProcessImmediateMeshes(assets, 16)

		client.Update(&camera)

		// Update Entity Interpolation
		for _, e := range remoteEntities {
			lerpFactor := float64(dt * 10.0) // 10 units per second speed-ish
			if lerpFactor > 1.0 {
				lerpFactor = 1.0
			}
			e.X += (e.TX - e.X) * lerpFactor
			e.Y += (e.TY - e.Y) * lerpFactor
			e.Z += (e.TZ - e.Z) * lerpFactor
		}

		unloadTimer += dt
		if unloadTimer >= unloadInterval {
			unloadTimer = 0
			// Unload logic...
			// Check server radius? For now local simple unload.
			cx := int(math.Floor(float64(camera.Position.X) / 16.0))
			cz := int(math.Floor(float64(camera.Position.Z) / 16.0))
			world.UnloadChunks(cx, cz, unloadRadius, func(ux, uz int) {
				client.Send(&PacketUnloadChunk{CX: int32(ux), CZ: int32(uz)})
			})
		}

		autosaveTimer += dt
		if autosaveTimer >= autosaveInterval {
			// Client shouldn't save to disk in multiplayer.
			// But for "Singleplayer via Internal Server", Server should save.
			// Currently Server doesn't have Save logic implemented yet (TODO).
			// We skip client saving.
			autosaveTimer = 0
		}

		camBlockX := int(math.Floor(float64(camera.Position.X) + 0.5))
		camBlockY := int(math.Floor(float64(camera.Position.Y) + 0.5))
		camBlockZ := int(math.Floor(float64(camera.Position.Z) + 0.5))
		inWater := world.BlockAt(camBlockX, camBlockY, camBlockZ) == blockWater

		rl.BeginDrawing()
		background := rl.NewColor(180, 210, 255, 255)
		if inWater {
			background = rl.NewColor(40, 70, 120, 255)
		}
		rl.ClearBackground(background)
		rl.BeginMode3D(camera)

		world.Draw(assets, camera)

		if hit.hit {
			pos := rl.NewVector3(float32(hit.x), float32(hit.y), float32(hit.z))
			rl.DrawCubeWires(pos, 1.02, 1.02, 1.02, rl.Red)
		}

		// Render Entities
		for id, e := range remoteEntities {
			if id == *username {
				continue // Don't draw self (avoids delayed reflection and blocking view)
			}
			if e.Type == EntityPlayer {
				drawCharacterModel(rl.NewVector3(float32(e.X), float32(e.Y), float32(e.Z)), e.Yaw)
			} else {
				// Simple Cube for Creatures
				pos := rl.NewVector3(float32(e.X), float32(e.Y)+0.5, float32(e.Z))
				rl.DrawCube(pos, 0.8, 0.8, 0.8, rl.Pink)
				rl.DrawCubeWires(pos, 0.8, 0.8, 0.8, rl.Black)
			}
		}

		rl.EndMode3D()

		if inWater {
			overlay := rl.NewColor(40, 90, 160, 120)
			rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), overlay)
		}

		assets.drawCrosshair()
		scale := uiScale()
		fontSize := int32(14 * scale)
		padding := int32(10 * scale)
		line := int32(18 * scale)

		if input.ShowDebug {
			rl.DrawText(fmt.Sprintf("Pos: %.1f, %.1f, %.1f", camera.Position.X, camera.Position.Y, camera.Position.Z),
				padding, padding, fontSize, rl.DarkGray)
			rl.DrawFPS(padding, padding+line)

			// Performance Stats
			stats := fmt.Sprintf("Frame Time: %.2f ms", perfMon.metrics.FrameTime)
			rl.DrawText(stats, padding, padding+line*2, fontSize, rl.DarkGray)

			memStats := fmt.Sprintf("Mem: %d MB  GC: %d", perfMon.metrics.HeapAllocMB, perfMon.metrics.NumGC)
			rl.DrawText(memStats, padding, padding+line*3, fontSize, rl.DarkGray)

			loadStats := fmt.Sprintf("Chunks: %d/%d (L/U/s: %d/%d)",
				perfMon.metrics.ChunksPerSec, perfMon.metrics.UnloadsPerSec,
				perfMon.chunksLoaded, perfMon.chunksUnloaded) // Accumulators
			rl.DrawText(loadStats, padding, padding+line*4, fontSize, rl.DarkGray)
		}
		if input.InventoryOpen {
			assets.drawInventory(input)
		} else {
			assets.drawHotbar(input)
		}

		rl.EndDrawing()
	}

	fmt.Println("Exiting... Saving state.")
	// 1. Signal Server to save world (Only if we are the host)
	if server != nil {
		server.Shutdown <- true
		time.Sleep(200 * time.Millisecond)
	}

	client.Conn.Close()
}

func drawCharacterModel(pos rl.Vector3, yaw float32) {
	rl.PushMatrix()
	rl.Translatef(pos.X, pos.Y, pos.Z)
	rl.Rotatef(-yaw, 0, 1, 0) // Rotate model to match look dir (invert yaw for world space)

	// Body
	rl.DrawCube(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.DarkGray)
	rl.DrawCubeWires(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.Black)

	// Head
	rl.DrawCube(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.LightGray)
	rl.DrawCubeWires(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.Black)

	// Hands (Floating)
	bob := float32(math.Sin(float64(rl.GetTime())*5)) * 0.1
	rl.DrawCube(rl.NewVector3(0.45, 0.7+bob, 0), 0.2, 0.2, 0.2, rl.Blue)
	rl.DrawCube(rl.NewVector3(-0.45, 0.7-bob, 0), 0.2, 0.2, 0.2, rl.Blue)

	rl.PopMatrix()
}
