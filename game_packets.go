package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func handlePacket(pkt Packet) {
	switch p := pkt.(type) {
	case *PacketVitals:
		wasDead := input.isDead()
		if input.VitalsReady && p.Health < input.Vitals.Health {
			input.HurtFlash = 0.45
		}
		input.Vitals = *p
		input.VitalsReady = true
		if input.isDead() {
			input.closeContainerUI()
			input.InventoryOpen = false
			input.CraftingStation = 0
			input.MiningProgress = 0
			input.MiningTarget = nil
			input.VelocityY = 0
			isChatOpen = false
			isPaused = false
			if server != nil {
				server.Paused.Store(false)
			}
			rl.EnableCursor()
		} else if wasDead {
			input.RespawnWaiting = false
			input.SkipCamera = true
			rl.DisableCursor()
		}
	case *PacketContainerState:
		if p.Token <= input.ClosedContainerToken {
			return
		}
		if p.State.Kind == 0 {
			if input.Container != nil && input.Container.Token == p.Token {
				input.Container = nil
				input.InventoryOpen = false
				input.SkipCamera = true
				if !isPaused {
					rl.DisableCursor()
				}
			}
		} else {
			input.Container = p
			input.InventoryOpen = true
			input.CraftingStation = 0
			input.SkipCamera = true
			rl.EnableCursor()
		}
	case *PacketOpenWindow:
		if p.WindowType == 1 { // Workbench
			input.closeContainerUI()
			input.InventoryOpen = true
			input.CraftingStation = blockCraftingTable
			input.SkipCamera = true
			rl.EnableCursor()

			// Center the mouse so it feels natural when UI opens
			rl.SetMousePosition(rl.GetScreenWidth()/2, rl.GetScreenHeight()/2)
		}

	case *PacketChunkData:
		if world.applyChunkPacket(p) {
			delete(pendingChunkRequests, chunkKey{int(p.CX), int(p.CZ)})
		}
	case *PacketChunkLight:
		world.applyChunkLight(p)

	case *PacketBlockChange:
		world.SetBlockAt(int(p.X), int(p.Y), int(p.Z), p.BlockID)
		world.SetMetaAt(int(p.X), int(p.Y), int(p.Z), p.Meta)

	case *PacketLogin:
		world.seed = p.Seed
		fmt.Printf("Synced with server seed: %d\n", p.Seed)

	case *PacketSpawnPoint:
		input.AwaitingTerrain = true
		camera.Position = rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
		camera.Target = rl.NewVector3(camera.Position.X, camera.Position.Y-2, camera.Position.Z+5)
		input.InitFromCamera(camera)
		input.VelocityY = 0
		input.OnGround = false
		input.IsRunning = false
		input.SprintLatched = false
		input.SkipCamera = true
		if client != nil {
			client.LastSentX = p.X
			client.LastSentY = p.Y
			client.LastSentZ = p.Z
		}

	case *PacketMobState:
		e := remoteEntities[p.IDString]
		if e == nil {
			e = &RemoteEntity{ID: p.IDString, Type: EntityPig, X: p.X, Y: p.Y, Z: p.Z, Yaw: p.Yaw}
			remoteEntities[p.IDString] = e
		}
		e.MobKind, e.MobState, e.MobHealth = p.Kind, p.State, p.Health
		e.TX, e.TY, e.TZ = p.X, p.Y, p.Z
		e.TargetYaw = p.Yaw
		e.MobHurt = float32(p.Hurt) / 20
		if p.Health <= 0 {
			e.DeathTime = max(e.DeathTime, float32(p.Death)/20)
		}
	case *PacketEntitySpawn:
		remoteEntities[p.EntityID] = &RemoteEntity{
			ID:   p.EntityID,
			Type: p.Type, // THIS WAS MISSING!
			X:    p.X, Y: p.Y, Z: p.Z,
			TX: p.X, TY: p.Y, TZ: p.Z,
			Yaw: p.Yaw, Pitch: p.Pitch,
			Metadata: p.Metadata,
		}
	case *PacketEntityDespawn:
		delete(remoteEntities, p.EntityID)
	case *PacketEntityMove:
		if e, ok := remoteEntities[p.EntityID]; ok {
			if e.MobKind != "" {
				return
			}
			e.TX, e.TY, e.TZ = p.X, p.Y, p.Z
			e.Yaw, e.Pitch = p.Yaw, p.Pitch
		}

	case *PacketPlayerMove:
		// Server forcing position (Teleport)
		// Usually client is authoritative, but if server sends it, we should respect.
		// Update camera immediately.
		camera.Position = rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
		// Reset interpolation or smoothing?
		// Also update last sent to avoid loop
		client.LastSentX = p.X
		client.LastSentY = p.Y
		client.LastSentZ = p.Z
		// We don't change Yaw/Pitch if they are 0 (which server sends on TP), unless we want to reset view.
		// server sent 0,0. Let's keep view for now unless flag is set. Simple TP usually keeps rotation or sets it.
		// Server code sent 0,0. Let's ignore rotation if 0,0? Or set it?
		// Let's just set position.

	case *PacketChat:
		// Add to history
		chatHistory = append(chatHistory, p.Message)
		// Keep max 10
		if len(chatHistory) > 10 {
			chatHistory = chatHistory[len(chatHistory)-10:]
		}
	case *PacketEntityMeta:
		if e, ok := remoteEntities[p.EntityID]; ok {
			e.Metadata = p.Metadata
		}
	case *PacketGameMode:
		currentGameMode = p.Mode
		fmt.Printf("GameMode switched to %d\n", p.Mode)

	case *PacketSlotChange:
		if p.Slot >= 0 && p.Slot < 9 {
			input.SelectedSlot = int(p.Slot)
			input.CurrentBlock = input.Hotbar[input.SelectedSlot]
		}
	case *PacketInventoryUpdate:
		if p.SlotID == -1 {
			// Update Cursor Item (Held on mouse)
			input.CursorItem = Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
		} else if p.SlotID >= 0 && p.SlotID < 36 {
			// Update the authoritative inventory
			if client != nil {
				client.Inventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
			} else {
				localInventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
			}
			// Sync Hotbar input state
			if p.SlotID < 9 {
				input.Hotbar[p.SlotID] = byte(p.ItemID)
				if input.SelectedSlot == int(p.SlotID) {
					input.CurrentBlock = byte(p.ItemID)
				}
			}
		}
	}
}
