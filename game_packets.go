package main

import (
	"fmt"
	"time"
)

func handlePacket(pkt Packet) {
	switch p := pkt.(type) {
	case *PacketExplosion:
		if world != nil {
			for _, pos := range p.Removed {
				x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
				if world.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth)) != nil {
					world.SetBlockAt(x, y, z, blockAir)
				}
			}
		}
		addExplosionEffect(p)
		if camera.Position != (gameVec3{}) {
			dx, dy, dz := float64(camera.Position.X)-p.X, float64(camera.Position.Y)-p.Y, float64(camera.Position.Z)-p.Z
			if dx*dx+dy*dy+dz*dz < 48*48 {
				playGameSound(soundExplosion)
			}
		}
	case *PacketWorldTime:
		if world != nil {
			world.TimeTicks = float64(p.Ticks)
		}
	case *PacketVitals:
		wasDead := input.isDead()
		if input.VitalsReady {
			if p.Food > input.Vitals.Food {
				playGameSound(soundEat)
			} else if p.Health < input.Vitals.Health {
				playGameSound(soundHurt)
			}
		}
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
			releaseCursor()
		} else if wasDead {
			input.RespawnWaiting = false
			input.SkipCamera = true
			captureCursor()
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
					captureCursor()
				}
			}
		} else {
			input.Container = p
			input.InventoryOpen = true
			input.CraftingStation = 0
			input.SkipCamera = true
			releaseCursor()
		}
	case *PacketOpenWindow:
		if p.WindowType == 1 { // Workbench
			input.closeContainerUI()
			input.InventoryOpen = true
			input.CraftingStation = blockCraftingTable
			input.SkipCamera = true
			releaseCursor()

			// Center the mouse so it feels natural when UI opens
			positionCursor(windowWidth()/2, windowHeight()/2)
		}

	case *PacketChunkData:
		if world.applyChunkPacket(p) {
			key := chunkKey{int(p.CX), int(p.CZ)}
			if _, pending := pendingChunkRequests[key]; pending {
				if start, ok := chunkLoadStarts[key]; ok && perfMon != nil {
					perfMon.receiveLatency = append(perfMon.receiveLatency, float64(time.Since(start))/float64(time.Millisecond))
				}
			}
			delete(pendingChunkRequests, key)
			recordChunkReady(key, world.getChunkIfGenerated(key.X, key.Z))
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
		camera.Position = newGameVec3(float32(p.X), float32(p.Y), float32(p.Z))
		camera.Target = newGameVec3(camera.Position.X, camera.Position.Y-2, camera.Position.Z+5)
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
		// Preserve the current view direction when the server teleports the player.
		next := newGameVec3(float32(p.X), float32(p.Y), float32(p.Z))
		camera.Target = gameVec3Add(camera.Target, gameVec3Subtract(next, camera.Position))
		camera.Position = next
		client.LastSentX = p.X
		client.LastSentY = p.Y
		client.LastSentZ = p.Z

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
