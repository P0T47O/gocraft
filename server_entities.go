package main

func (s *Server) SendInventory(player *PlayerEntity) {
	player.claimPendingItems()
	// Sync entire inventory to client
	// We MUST send empty slots (ID=0) too, otherwise client won't know items were consumed!
	for i, item := range player.Inventory.Slots {
		p := &PacketInventoryUpdate{
			SlotID: int32(i),
			ItemID: item.ID,
			Count:  item.Count,
			Damage: item.Damage,
		}
		s.BroadcastTo(player.UUID, p)
	}
}

func (s *Server) UpdateEntities() {
	s.World.TickEntities()

	// Handle Item Pickup & Remove dead entities
	s.World.entitiesMu.Lock()
	var toRemove []string
	var arrowHits []mobAttack
	inventoryDirty := make(map[*PlayerEntity]bool)

	// Collect Players for distance check
	var players []*PlayerEntity
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok && !p.dead() {
			if p.Vitals != nil {
				s.ClientsMu.RLock()
				online := s.Clients[p.UUID] != nil
				s.ClientsMu.RUnlock()
				if !online {
					continue
				}
			}
			players = append(players, p)
		}
	}

	for _, e := range s.World.entities {
		if arrow, ok := e.(*ArrowEntity); ok {
			if arrow.HitPlayer != nil {
				arrowHits = append(arrowHits, mobAttack{arrow.HitPlayer, arrow.Damage, "Shot by skeleton"})
			}
			if arrow.Dead {
				toRemove = append(toRemove, arrow.UUID)
			}
			continue
		}
		// Item Pickup Logic
		if item, ok := e.(*ItemEntity); ok && !item.Dead && item.PickupDelay <= 0 {
			for _, player := range players {
				// Distance Check (Radius 1.5)
				dx := player.X - item.X
				dy := player.Y - item.Y
				dz := player.Z - item.Z
				distSq := dx*dx + dy*dy + dz*dz

				if distSq < 2.25 {
					// Try add to inventory
					rem := player.Inventory.AddStack(item.ItemStack)
					if rem < int32(item.Count) {
						// Some or all picked up. Multiple nearby drops can be
						// collected in one server tick (death piles and water often
						// cluster them), so defer the full 36-slot sync and send it
						// once per affected player after the pickup pass.
						inventoryDirty[player] = true

						if rem == 0 {
							item.Dead = true
						} else {
							item.Count = rem
							// Update count for others
							meta := item.ID | (item.Count << 8) | (item.Damage << 16)
							s.Broadcast(&PacketEntityMeta{
								EntityID: item.GetUUID(),
								Metadata: meta,
							})
						}
					}
				}
			}
		}

		if item, ok := e.(*ItemEntity); ok && item.Dead {
			toRemove = append(toRemove, e.GetUUID())
		}
	}
	for _, uuid := range toRemove {
		for i, e := range s.World.entities {
			if e.GetUUID() == uuid {
				s.World.entities = append(s.World.entities[:i], s.World.entities[i+1:]...)
				break
			}
		}
		s.Broadcast(&PacketEntityDespawn{EntityID: uuid})
		delete(s.LastSentPos, uuid)
		delete(s.LastSentMeta, uuid)
	}
	s.World.entitiesMu.Unlock()
	for _, hit := range arrowHits {
		s.hurtPlayer(hit.player, hit.amount, hit.cause)
	}

	// A full inventory snapshot is 36 authoritative packets. Sending one for
	// every item collected in the same tick can overflow the 128-packet gameplay
	// queue even though the final inventory state only needs one snapshot.
	for player := range inventoryDirty {
		s.SendInventory(player)
	}

	s.World.entitiesMu.RLock()
	defer s.World.entitiesMu.RUnlock()

	for _, e := range s.World.entities {
		if e.IsDirty() {
			x, y, z := e.GetPosition()

			// Threshold check
			last, ok := s.LastSentPos[e.GetUUID()]
			dx := x - last[0]
			dy := y - last[1]
			dz := z - last[2]
			distSq := dx*dx + dy*dy + dz*dz

			if !ok || distSq > 0.0025 { // > 0.05 units
				yaw, pitch := e.GetRotation()
				move := &PacketEntityMove{
					EntityID: e.GetUUID(),
					X:        x,
					Y:        y,
					Z:        z,
					Yaw:      yaw,
					Pitch:    pitch,
				}
				s.Broadcast(move)
				s.LastSentPos[e.GetUUID()] = [3]float64{x, y, z}
			}

			// Update metadata for items (count changed)
			if item, ok := e.(*ItemEntity); ok {
				meta := item.ID | (item.Count << 8) | (item.Damage << 16)
				lastMeta, hasLast := s.LastSentMeta[e.GetUUID()]
				if !hasLast || lastMeta != meta {
					s.Broadcast(&PacketEntityMeta{
						EntityID: e.GetUUID(),
						Metadata: meta,
					})
					s.LastSentMeta[e.GetUUID()] = meta
				}
			}

			e.ClearDirty()
		}
	}
}

func (s *Server) SpawnEntity(e Entity) {
	s.World.entitiesMu.Lock()
	s.World.entities = append(s.World.entities, e)
	s.World.entitiesMu.Unlock()

	x, y, z := e.GetPosition()
	yaw, pitch := e.GetRotation()

	meta := int32(0)
	if item, ok := e.(*ItemEntity); ok {
		meta = item.ID | (item.Count << 8) | (item.Damage << 16)
	}

	s.Broadcast(&PacketEntitySpawn{
		EntityID: e.GetUUID(),
		Type:     e.GetType(),
		X:        x,
		Y:        y,
		Z:        z,
		Yaw:      yaw,
		Pitch:    pitch,
		Metadata: meta,
	})
	if m, ok := e.(*MobEntity); ok {
		s.Broadcast(m.snapshot())
	}
}

// findPlayerEntity finds a PlayerEntity by UUID. Caller must hold entitiesMu (RLock or Lock).
func (s *Server) findPlayerEntity(uuid string) *PlayerEntity {
	for _, e := range s.World.entities {
		if e.GetUUID() == uuid {
			if p, ok := e.(*PlayerEntity); ok {
				return p
			}
			return nil
		}
	}
	return nil
}
