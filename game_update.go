package main

import (
	"math"
	"time"
)

func updateGame() {
	if isPaused && pauseSettings && inputKeyPressed(keyEscape) {
		SaveSettings()
		pauseSettings = false
		ui.ActiveID = ""
		return
	}
	if !input.isDead() {
		// Chat Input
		if isChatOpen {
			// Handle keys
			char := inputChar()
			for char > 0 {
				if char >= 32 && char <= 125 {
					chatInput += string(char)
				}
				char = inputChar()
			}

			if inputKeyPressed(keyBackspace) {
				if len(chatInput) > 0 {
					chatInput = chatInput[:len(chatInput)-1]
				}
			}

			if inputKeyPressed(keyEnter) {
				if len(chatInput) > 0 {
					client.Send(&PacketChat{Message: chatInput})
					chatInput = ""
				}
				isChatOpen = false
				captureCursor()
			}

			if inputKeyPressed(keyEscape) {
				isChatOpen = false
				captureCursor()
			}

			return // Block other inputs while chat is open
		} else if !isPaused && !input.InventoryOpen && inputKeyPressed(keyEnter) {
			isChatOpen = true
			releaseCursor()
			// positionCursor? No, let cursor be free.
			return
		}

		if inputKeyPressed(keyEscape) {
			if input.InventoryOpen {
				input.closeContainerUI()
				input.InventoryOpen = false
				input.CraftingStation = 0
				input.SkipCamera = true
				if !isPaused {
					captureCursor()
				}
			} else {
				isPaused = !isPaused
				if server != nil {
					server.Paused.Store(isPaused)
				}
				if isPaused {
					releaseCursor()
					ui.ActiveID = ""
				} else {
					captureCursor()
					// Reset mouse to center to prevent view jump
					positionCursor(windowWidth()/2, windowHeight()/2)
				}
			}
		}

	}

	// Inventory toggle is handled in HandleInput -> ToggleInventory

	// Singleplayer Pause: Freeze update loop
	if isPaused && server != nil {
		return
	}

	dt := gameFrameTime()
	assets.Update(dt)

	// Packet Loop
	packetDeadline := time.Now().Add(2 * time.Millisecond)
Loop:
	for packets := 0; packets < 64; packets++ {
		if packets > 0 && time.Now().After(packetDeadline) {
			break
		}
		select {
		case pkt, ok := <-client.Incoming:
			if !ok {
				exitGame()
				return
			}
			handlePacket(pkt)
		default:
			break Loop
		}
	}

	if isPaused {
		return
	} // Keep receiving multiplayer packets without gameplay input.

	world.ProcessMeshResults(assets, 16)

	if input.isDead() {
		if *useWebGPU && !input.RespawnWaiting && inputKeyPressed(keyR) {
			input.RespawnWaiting = true
			input.RespawnLast = 0
		}
		input.updateRespawnRequest(client)
		requestMissingChunks(camera.Position)
		return
	}
	if input.AwaitingTerrain {
		cx := divFloor(blockIndexFromCoord(camera.Position.X), chunkWidth)
		cz := divFloor(blockIndexFromCoord(camera.Position.Z), chunkWidth)
		if world.getChunkIfGenerated(cx, cz) == nil {
			requestMissingChunks(camera.Position)
			return
		}
		input.AwaitingTerrain = false
	}
	if !input.VitalsReady {
		requestMissingChunks(camera.Position)
		return
	}
	input.HurtFlash = max(float32(0), input.HurtFlash-dt)

	HandleInput(world, &camera, input, client)
	world.ProcessImmediateMeshes(assets, 16)
	clear(world.lightChanged) // Only the server publishes authoritative light updates.

	// Client-Pull: Request any missing chunks
	requestMissingChunks(camera.Position)

	client.Update(camera.Position.X, camera.Position.Y, camera.Position.Z, input.Yaw, input.Pitch)

	updateEntities(dt)
	updateInterpolation(dt)

	// Clean up far chunks (Client-side Garbage Collection)
	// Render radius is roughly 16. Keep a bit more (e.g. 20) to avoid thrashing.
	// 5 seconds interval? Or every frame?
	// Every frame is fine, UnloadChunks is efficient enough (iterates map).
	// But let's do it every 60 frames to be safe on CPU.
	if dt > 0 {
		pPos := camera.Position
		cx := int(math.Floor(float64(pPos.X) / 16.0))
		cz := int(math.Floor(float64(pPos.Z) / 16.0))
		// Use a static counter to throttle
		// Accessing global or static var is ugly here, let's just run it. Map iteration of ~1000 items is fast.
		// Radius 24 chunks (16 render + 8 buffer)
		world.UnloadChunks(cx, cz, renderDistance()+4, func(chunkX, chunkZ int) {
			// Notify server that we unloaded this chunk
			// So it knows to resend if we return
			if client != nil {
				client.Send(&PacketUnloadChunk{CX: int32(chunkX), CZ: int32(chunkZ)})
			}
		})
	}
}
