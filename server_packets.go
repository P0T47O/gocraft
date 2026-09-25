package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

func (s *Server) HandlePacket(wrap PacketWrapper) {
	if wrap.Connection != nil {
		s.ClientsMu.RLock()
		current := s.Clients[wrap.From]
		s.ClientsMu.RUnlock()
		if current != wrap.Connection {
			return
		}
	}
	pkt := wrap.Packet
	if player := s.findPlayerEntity(wrap.From); player != nil && player.dead() {
		switch pkt.(type) {
		case *PacketRespawn, *PacketLogin, *PacketChunkRequest, *PacketUnloadChunk:
		default:
			return
		}
	}
	// client := s.Clients[wrap.From]

	switch p := pkt.(type) {
	case *PacketAttackMob:
		s.attackMob(s.findPlayerEntity(wrap.From), p.Target)
	case *PacketContainerClick:
		s.clickContainer(s.findPlayerEntity(wrap.From), p)
	case *PacketRespawn:
		if player := s.findPlayerEntity(wrap.From); player != nil {
			delete(s.miningSessions, wrap.From)
			s.respawnPlayer(player)
		}
	case *PacketLogin:
		delete(s.miningSessions, wrap.From)
		delete(s.ContainerSessions, wrap.From)
		fmt.Printf("Client %s logged in on protocol %d (Seed: %d)\n", p.Username, p.ProtocolVersion, s.World.seed)
		// Update packet with server seed so client can sync if desired
		p.Seed = s.World.seed

		// Register Client if not exists
		s.ClientsMu.Lock()
		if _, ok := s.Clients[p.Username]; !ok {
			conn := &ClientConnection{
				Name:        p.Username,
				Send:        make(chan Packet, 128),
				KnownChunks: make(map[chunkKey]bool),
			}
			s.Clients[p.Username] = conn
		}
		s.ClientsMu.Unlock()

		// Send the seed before any chunk snapshots or meshes.
		s.BroadcastTo(p.Username, p)
		s.BroadcastTo(p.Username, &PacketWorldTime{Ticks: int64(s.World.TimeTicks)})

		// 2. Send Spawn Point
		var spawnX, spawnY, spawnZ float64
		if s.HasSavedPos {
			spawnX, spawnY, spawnZ = s.InitialPosX, s.InitialPosY, s.InitialPosZ
		} else {
			sx, sz, ok := findLandSpawn(s.World, 0, 0, 16)
			if !ok {
				sx, sz = 0, 0
			}
			spawnX, spawnY, spawnZ = float64(sx), float64(s.World.HeightAt(sx, sz))+2.0, float64(sz)
		}

		if saved := s.findPlayerEntity(p.Username); saved != nil {
			if !s.LocalCheats {
				saved.GameMode = ModeSurvival
			}
			spawnX, spawnY, spawnZ = saved.X, saved.Y, saved.Z
		}
		s.ClientsMu.RLock()
		client, ok := s.Clients[p.Username]
		s.ClientsMu.RUnlock()

		if ok {
			client.LastChunkX = int(math.Floor(spawnX / chunkWidth))
			client.LastChunkZ = int(math.Floor(spawnZ / chunkWidth))
			client.enqueue(&PacketSpawnPoint{
				X: spawnX,
				Y: spawnY,
				Z: spawnZ,
			})
			s.SendChunksAround(p.Username, client.LastChunkX, client.LastChunkZ, 16)
		}

		// 3. Send Existing Entities
		s.World.entitiesMu.RLock()
		for _, e := range s.World.entities {
			if e.GetUUID() == p.Username {
				continue
			} // Skip self if represented as entity
			ex, ey, ez := e.GetPosition()
			eyaw, epitch := e.GetRotation()

			meta := int32(0)
			if item, ok := e.(*ItemEntity); ok {
				meta = item.ID | (item.Count << 8) | (item.Damage << 16)
			}

			s.ClientsMu.RLock()
			client, ok := s.Clients[p.Username]
			s.ClientsMu.RUnlock()
			if ok {
				client.enqueue(&PacketEntitySpawn{
					EntityID: e.GetUUID(),
					Type:     e.GetType(),
					X:        ex,
					Y:        ey,
					Z:        ez,
					Yaw:      eyaw,
					Pitch:    epitch,
					Metadata: meta,
				})
				if m, ok := e.(*MobEntity); ok {
					client.enqueue(m.snapshot())
				}
			}
		}
		s.World.entitiesMu.RUnlock()

		// 4. Register new player as entity and broadcast to others
		// Check if entity already exists (rejoining)
		var exists bool
		s.World.entitiesMu.RLock()
		for _, e := range s.World.entities {
			if e.GetUUID() == p.Username {
				exists = true
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if !exists {
			playerEnt := &PlayerEntity{
				GameMode: ModeSurvival,
				BaseEntity: BaseEntity{
					UUID: p.Username, Type: EntityPlayer,
					X: spawnX, Y: spawnY, Z: spawnZ,
				},
			}
			s.SpawnEntity(playerEnt)
		} else {
			fmt.Printf("Player %s rejoining existing entity\n", p.Username)
		}

		if player := s.findPlayerEntity(p.Username); player != nil {
			player.initVitals()
			player.Vitals.grace = 60
			s.sendVitals(player)
			s.SendInventory(player)
			if len(player.PendingItems) > 0 {
				var count int32
				for _, stack := range player.PendingItems {
					count += stack.Count
				}
				s.SendTo(p.Username, &PacketChat{Message: fmt.Sprintf("%d migrated items are safe in storage; free inventory slots to receive them.", count)})
			}
			s.BroadcastTo(p.Username, &PacketInventoryUpdate{SlotID: -1, ItemID: player.CursorItem.ID, Count: player.CursorItem.Count, Damage: player.CursorItem.Damage})
			s.BroadcastTo(p.Username, &PacketSlotChange{Slot: int32(player.SelectedSlot)})
			s.BroadcastTo(p.Username, &PacketGameMode{Mode: player.GameMode})
		}

	case *PacketGameMode:
		if p.Mode > ModeSurvival {
			return
		}
		// Switch GameMode
		s.World.entitiesMu.RLock()
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if pEnt, ok := e.(*PlayerEntity); ok && pEnt.UUID == wrap.From {
				player = pEnt
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player != nil {
			if !s.LocalCheats {
				s.SendTo(wrap.From, &PacketGameMode{Mode: player.GameMode})
				s.SendTo(wrap.From, &PacketChat{Message: "Game mode switching is disabled on this server."})
				return
			}
			delete(s.miningSessions, wrap.From)
			player.GameMode = p.Mode
			if player.Vitals != nil {
				player.Vitals.Air = maxAir
				player.Vitals.FireTicks = 0
				player.Vitals.FallDistance = 0
				player.Vitals.lastY = player.Y
				s.sendVitals(player)
			}
			fmt.Printf("Player %s switched to GameMode %d\n", wrap.From, p.Mode)
			// Echo back to confirm (or broadcast if others need to know)
			s.BroadcastTo(wrap.From, p)
		}

	case *PacketPlayerMove:
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsNaN(p.Z) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || math.IsInf(p.Z, 0) {
			return
		}
		// Update player position in ServerWorld for saving
		s.InitialPosX = p.X
		s.InitialPosY = p.Y
		s.InitialPosZ = p.Z
		s.HasSavedPos = true

		// Update Player Entity in world for broadcasting
		s.World.entitiesMu.Lock()
		if player := s.findPlayerEntity(wrap.From); player != nil {
			player.SetPosition(p.X, p.Y, p.Z)
			player.SetRotation(p.Yaw, p.Pitch)
		}
		s.World.entitiesMu.Unlock()

		cx := int(math.Floor(p.X / 16.0))
		cz := int(math.Floor(p.Z / 16.0))

		s.ClientsMu.RLock()
		client, ok := s.Clients[wrap.From]
		s.ClientsMu.RUnlock()
		if ok {
			if cx != client.LastChunkX || cz != client.LastChunkZ {
				client.LastChunkX = cx
				client.LastChunkZ = cz
				s.SendChunksAround(client.Name, cx, cz, 16)
			}
		}

	case *PacketBlockInteract:
		pos := BlockPos{p.X, p.Y, p.Z}
		player := s.findPlayerEntity(wrap.From)
		if player == nil {
			return
		}
		switch p.Action {
		case 1: // Begin mining.
			s.beginMining(player, pos, time.Now())
			return
		case 2: // Cancel mining.
			if session, ok := s.miningSessions[wrap.From]; ok && session.pos == pos {
				delete(s.miningSessions, wrap.From)
			}
			return
		case 0: // Container / crafting interaction.
		default:
			return
		}
		if !s.containerReach(player, pos) {
			return
		}
		if s.interactFarm(player, pos) {
			return
		}
		if s.World.BlockAt(int(p.X), int(p.Y), int(p.Z)) == blockTNT {
			s.igniteTNT(int(p.X), int(p.Y), int(p.Z), tntFuseTicks)
			return
		}
		if containerSize(s.World.BlockAt(int(p.X), int(p.Y), int(p.Z))) > 0 {
			s.openContainer(s.findPlayerEntity(wrap.From), pos)
			return
		}
		// Check for specific block interactions (e.g. Workbench)
		blockID := s.World.BlockAt(int(p.X), int(p.Y), int(p.Z))

		// If Workbench, send OpenWindow
		if blockID == blockCraftingTable {
			// Send OpenWindow packet
			s.ClientsMu.RLock()
			client, ok := s.Clients[wrap.From]
			s.ClientsMu.RUnlock()
			if ok {
				// WindowID 1, Type 1 (Workbench) - Type 0 is Inventory/Hand
				// Wait, let's use Type=1 for Workbench as per plan
				client.enqueue(&PacketOpenWindow{
					WindowID:   1,
					WindowType: 1, // 1 = Workbench
				})
			}
		}

	case *PacketCraft:
		// Handle Crafting Request
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()

		if player != nil {
			// 1. Get Recipe
			if p.RecipeID < 0 || int(p.RecipeID) >= len(RecipeRegistry) {
				return
			}
			recipe := RecipeRegistry[p.RecipeID]

			if !s.hasCraftingStation(player, recipe.Station) {
				return
			}
			count := int(p.Count)
			if count == 0 {
				count = 1
			} // Older clients craft one batch.
			player.Inventory.Craft(recipe, count)
			s.SendInventory(player)
		}

	case *PacketChat:
		// Clean message
		msg := p.Message
		if len(msg) > 0 {
			if msg[0] == '/' {
				s.handleCommand(wrap.From, msg)
			} else {
				// Broadcast
				formatted := fmt.Sprintf("<%s> %s", wrap.From, msg)
				fmt.Println("Chat: " + formatted)
				s.Broadcast(&PacketChat{Message: formatted})
			}
		}

	case *PacketBlockChange:
		// Attempt to place/break block
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()

		if player == nil {
			return
		}
		pos := BlockPos{p.X, p.Y, p.Z}
		if p.Y < 0 || p.Y >= chunkHeight {
			return
		}
		if !s.blockChangeReach(player, pos) {
			s.SendTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: s.World.BlockAt(int(p.X), int(p.Y), int(p.Z)), Meta: s.World.MetaAt(int(p.X), int(p.Y), int(p.Z))})
			return
		}

		old := s.World.BlockAt(int(p.X), int(p.Y), int(p.Z))
		if p.BlockID != blockAir && (p.BlockID >= 100 || GetBlock(p.BlockID).ID == blockAir || (old != blockAir && old != blockWater)) {
			s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: old, Meta: s.World.MetaAt(int(p.X), int(p.Y), int(p.Z))})
			return
		}
		if p.BlockID == blockFarmland || isCrop(p.BlockID) {
			s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: old, Meta: s.World.MetaAt(int(p.X), int(p.Y), int(p.Z))})
			return
		}
		if p.BlockID != blockAir {
			// Placement Logic
			if player.GameMode == ModeSurvival {
				// 1. Try to consume from selected slot FIRST
				consumed := false
				slotIdx := player.SelectedSlot
				if slotIdx >= 0 && slotIdx < 36 {
					slot := &player.Inventory.Slots[slotIdx]
					if slot.ID == int32(p.BlockID) && slot.Count > 0 {
						slot.Count--
						if slot.Count == 0 {
							slot.ID = 0
						}
						consumed = true
					}
				}

				// 2. Fallback to general consume if not found in hand
				if !consumed {
					if !player.Inventory.Consume(int32(p.BlockID), 1) {
						// Failed to consume (cheating? lag?), revert client block
						fmt.Printf("Player %s tried to place Block %d without item.\n", wrap.From, p.BlockID)
						s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: old})
						return
					}
				}
				// Sync Inventory
				s.SendInventory(player)
			}
		} else {
			// Block Break Logic (Mining)
			// Check drops BEFORE setting to Air
			oldBlockID := s.World.BlockAt(int(p.X), int(p.Y), int(p.Z))
			if player.GameMode == ModeSurvival && !s.mayFinishMining(player, pos, oldBlockID, time.Now()) {
				s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: oldBlockID, Meta: s.World.MetaAt(int(p.X), int(p.Y), int(p.Z))})
				return
			}
			if oldBlockID != blockAir {
				blockDef := GetBlock(oldBlockID)
				if player.GameMode == ModeSurvival && blockDef.Hardness < 0 {
					s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: oldBlockID})
					return
				}
				if player.GameMode == ModeSurvival {
					if isCrop(oldBlockID) {
						s.dropCrop(int(p.X), int(p.Y), int(p.Z), oldBlockID, s.World.MetaAt(int(p.X), int(p.Y), int(p.Z)))
					} else if oldBlockID == blockTallGrass && rand.Intn(4) == 0 {
						s.dropFarmItem(int(p.X), int(p.Y), int(p.Z), itemWheatSeeds, 1)
					}
				}

				heldID := byte(0)
				slotIdx := player.SelectedSlot
				if slotIdx >= 0 && slotIdx < 9 && player.Inventory.Slots[slotIdx].Count > 0 {
					heldID = byte(player.Inventory.Slots[slotIdx].ID)
				}
				canHarvest := !blockDef.NoDrop && CanHarvest(heldID, oldBlockID)

				// 3. Drop Item
				if canHarvest && player.GameMode == ModeSurvival {
					dropID := blockDef.DropItem
					if dropID == 0 {
						dropID = blockDef.ID // Drop self by default
					}

					count := blockDef.DropCount
					if count <= 0 {
						count = 1
					}

					if dropID != 0 {
						// Spawn Item Entity
						// Random velocity
						rng := rand.New(rand.NewSource(time.Now().UnixNano()))
						vx := (rng.Float64() - 0.5) * 0.1
						vz := (rng.Float64() - 0.5) * 0.1
						vy := 0.2 // Slight pop up

						item := &ItemEntity{
							BaseEntity: BaseEntity{
								UUID: fmt.Sprintf("drop-%d-%d", time.Now().UnixNano(), rand.Int()),
								Type: EntityItem,
								X:    float64(p.X) + 0.5,
								Y:    float64(p.Y) + 0.3,
								Z:    float64(p.Z) + 0.5,
							},
							ItemStack:   ItemStack{ID: int32(dropID), Count: int32(count)},
							Vx:          vx,
							Vy:          vy,
							Vz:          vz,
							PickupDelay: 0.5, // Reduced delay for mined items
							Age:         0,
						}
						s.SpawnEntity(item)
					}
				}
				if player.GameMode == ModeSurvival && blockDef.Hardness > 0 && slotIdx >= 0 && slotIdx < 9 {
					player.Inventory.Slots[slotIdx].Wear(1)
					s.SendInventory(player)
				}

			}
		}

		if p.BlockID == blockAir {
			s.breakContainer(BlockPos{p.X, p.Y, p.Z})
			if old == blockFarmland {
				x, y, z := int(p.X), int(p.Y)+1, int(p.Z)
				crop := s.World.BlockAt(x, y, z)
				if isCrop(crop) {
					if player.GameMode == ModeSurvival {
						s.dropCrop(x, y, z, crop, s.World.MetaAt(x, y, z))
					}
					s.setFarmBlock(x, y, z, blockAir, 0)
				}
			}
		}
		// Apply block change to World
		// Validation logic would go here
		fmt.Printf("Server: Block set at %d %d %d\n", p.X, p.Y, p.Z)
		s.World.SetBlockAt(int(p.X), int(p.Y), int(p.Z), p.BlockID)
		if p.BlockID != blockTorch || p.Meta > 4 {
			p.Meta = 0
		}
		s.World.SetMetaAt(int(p.X), int(p.Y), int(p.Z), p.Meta)
		s.scheduleFluidAround(BlockPos{p.X, p.Y, p.Z})

		// Broadcast to all clients (including sender for confirmation, or skip sender)
		// For now, simple echo to prove it works
		s.Broadcast(p)

	case *PacketUnloadChunk:
		s.ClientsMu.RLock()
		client, ok := s.Clients[wrap.From]
		s.ClientsMu.RUnlock()
		if ok {
			key := chunkKey{X: int(p.CX), Z: int(p.CZ)}
			delete(client.KnownChunks, key)
		}

	case *PacketInventoryUpdate:
		// Client synced inventory (Creative Pick or Sync)
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()

		if player != nil {
			delete(s.miningSessions, wrap.From)
			if player.GameMode != ModeCreative {
				s.SendInventory(player)
				return
			}
			incoming := Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
			if !validStack(incoming) {
				s.SendInventory(player)
				return
			}
			// Handle Cursor Update (Slot -1)
			if p.SlotID == -1 {
				// Only allow arbitrary cursor setting in Creative Mode?
				// For now let's allow it generally or check GameMode if strict.
				// Since we use this for robust sync, let's allow it.
				player.CursorItem = Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
			} else if p.SlotID >= 0 && p.SlotID < int32(len(player.Inventory.Slots)) && (p.SlotID < armorSlotStart || incoming.ID == 0 || armorFits(int(p.SlotID), incoming)) {
				player.Inventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count, Damage: p.Damage}
				// fmt.Printf("Server: Updated slot %d for %s to %d:%d\n", p.SlotID, wrap.From, p.ItemID, p.Count)
			}
		}

	case *PacketSlotChange:
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()

		if player != nil {
			if p.Slot >= 0 && p.Slot < 9 {
				if player.SelectedSlot != int(p.Slot) {
					delete(s.miningSessions, wrap.From)
				}
				player.SelectedSlot = int(p.Slot)
			}
		}

	case *PacketClickWindow:
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()
		if player != nil && !p.IsCreative {
			delete(s.miningSessions, wrap.From)
			player.Inventory.Click(int(p.SlotID), int(p.Button), &player.CursorItem)
			s.SendInventory(player)
			s.sendVitals(player)
			s.SendTo(wrap.From, &PacketInventoryUpdate{SlotID: -1, ItemID: player.CursorItem.ID, Count: player.CursorItem.Count, Damage: player.CursorItem.Damage})
		}

	case *PacketPlayerAction:
		if p.ActionType == 2 {
			s.eatSelectedFood(s.findPlayerEntity(wrap.From))
			return
		}
		if p.ActionType != 0 {
			return
		}
		// 1. Find player pos/rot
		s.World.entitiesMu.RLock()
		player := s.findPlayerEntity(wrap.From)
		s.World.entitiesMu.RUnlock()

		if player == nil {
			return
		}
		px, py, pz := player.GetPosition()
		yaw, pitch := player.GetRotation()

		// 2. Spawn Item
		itemID := byte(p.Value & 0xFF)
		count := int((p.Value >> 8) & 0xFF)
		if count <= 0 {
			count = 1
		}

		dropped := ItemStack{ID: int32(itemID), Count: int32(count)}
		if player.GameMode == ModeSurvival {
			if player.SelectedSlot < 0 || player.SelectedSlot >= 9 {
				return
			}
			slot := &player.Inventory.Slots[player.SelectedSlot]
			if slot.ID != int32(itemID) || slot.Count < int32(count) {
				s.SendInventory(player)
				return
			}
			dropped = ItemStack{}
			MoveStack(&dropped, slot, int32(count))

			s.SendInventory(player)
		} else {
			dropped.Count = min(dropped.Count, StackLimit(dropped.ID))
			if dropped.Count <= 0 {
				return
			}
		}

		// Calculate Toss Velocity (based on Camera Forward)
		// Matches input.go logic:
		// X = Sin(Yaw)*Cos(Pitch)
		// Y = Sin(Pitch)
		// Z = Cos(Yaw)*Cos(Pitch)
		radYaw := float64(yaw)
		radPitch := float64(pitch)

		dirX := math.Sin(radYaw) * math.Cos(radPitch)
		dirY := math.Sin(radPitch)
		dirZ := math.Cos(radYaw) * math.Cos(radPitch)

		speed := 0.5 // Normal speed

		item := &ItemEntity{
			BaseEntity: BaseEntity{
				UUID:  fmt.Sprintf("item-%d-%d", time.Now().UnixNano(), rand.Int()),
				Type:  EntityItem,
				X:     px,       // Spawn at eye position (adjust if needed, but 0.3 offset caused clipping if looking down)
				Y:     py - 0.1, // Slightly below eyes
				Z:     pz,
				Yaw:   0,
				Pitch: 0,
			},
			ItemStack:   dropped,
			Vx:          dirX * speed,
			Vy:          dirY * speed, // Follow look direction exactly
			Vz:          dirZ * speed,
			PickupDelay: 1.5,
			Age:         0,
		}
		s.SpawnEntity(item)

	case *PacketChunkRequest:
		// Client-Pull: Client requests a specific chunk
		cx, cz := int(p.CX), int(p.CZ)

		// Validate: Is the request within legal range?
		s.ClientsMu.RLock()
		client, ok := s.Clients[wrap.From]
		s.ClientsMu.RUnlock()

		if !ok {
			return
		}

		// Check distance from player's last known position
		dx := cx - client.LastChunkX
		dz := cz - client.LastChunkZ
		maxRange := maxRenderDistance + 2 // Bounded client-pull range; no UI state on server thread.

		if dx*dx+dz*dz > maxRange*maxRange {
			// Request is too far, ignore (anti-cheat)
			return
		}

		key := chunkKey{cx, cz}
		s.World.requestChunk(cx, cz)
		s.queueChunkFor(key, wrap.From)

	} // End of switch
} // End of HandlePacket
