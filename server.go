package main

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"
)

type Server struct {
	World     *World // The authoritative world
	Clients   map[string]*ClientConnection
	ClientsMu sync.RWMutex
	PacketCh  chan PacketWrapper
	Shutdown  chan bool
	Done      chan bool

	// Initial player state (Loaded from save)
	InitialPosX, InitialPosY, InitialPosZ float64
	HasSavedPos                           bool
	LastSentPos                           map[string][3]float64
	LastSentMeta                          map[string]int32
	SavePath                              string
	PendingChunks                         map[chunkKey][]string
	Listener                              net.Listener
}

type lastPos struct {
	x, y, z float64
}

type ClientConnection struct {
	Name        string
	Conn        net.Conn
	Send        chan Packet // Buffer for outgoing packets
	KnownChunks map[chunkKey]bool
	LastChunkX  int
	LastChunkZ  int
}

type PacketWrapper struct {
	Packet Packet
	From   string
}

func NewServer(savePath string) *Server {
	world := NewFlatWorld()
	// Authoritative Load (MUST BE BEFORE STARTING WORKERS)
	hasPos, px, py, pz, _ := LoadWorld(savePath, world)

	// Set save path for chunk loading in background workers
	world.SavePath = savePath

	// Now we can start the workers with the correct seed
	fmt.Printf("Server: Authoritative Seed Loaded: %d\n", world.seed)
	world.StartBackend()

	s := &Server{
		World:         world,
		Clients:       make(map[string]*ClientConnection),
		PacketCh:      make(chan PacketWrapper, 1024),
		Shutdown:      make(chan bool),
		Done:          make(chan bool),
		InitialPosX:   px,
		InitialPosY:   py,
		InitialPosZ:   pz,
		HasSavedPos:   hasPos,
		LastSentPos:   make(map[string][3]float64),
		LastSentMeta:  make(map[string]int32),
		SavePath:      savePath,
		PendingChunks: make(map[chunkKey][]string),
	}

	if !hasPos && len(world.entities) == 0 {
		// Spawn a starter pig
		p := &PigEntity{
			BaseEntity: BaseEntity{
				UUID: "Piggy-01",
				Type: EntityPig,
				X:    8,
				Y:    float64(world.HeightAt(8, 20)) + 1,
				Z:    20,
			},
		}
		s.SpawnEntity(p)
	}

	return s
}

func (s *Server) Stop() {
	if s.Shutdown != nil {
		select {
		case <-s.Shutdown:
			// Already closed
		default:
			close(s.Shutdown)
		}
	}
}

func (s *Server) Start() {
	fmt.Println("Server starting...")
	ticker := time.NewTicker(50 * time.Millisecond) // 20 TPS
	defer ticker.Stop()

	for {
		select {
		case <-s.Shutdown:
			s.Save()
			close(s.Done)
			return
		case <-ticker.C:
			s.Tick()
		case wrap := <-s.PacketCh:
			s.HandlePacket(wrap)
		}
	}
}

func (s *Server) Save() {
	fmt.Println("Server: Saving world state...")
	// 0. Close listener if active
	if s.Listener != nil {
		s.Listener.Close()
	}
	// 1. Save Chunks
	if err := SaveWorldChunks(s.SavePath, s.World); err != nil {
		fmt.Printf("Server Save Chunks Error: %v\n", err)
	}
	// 2. Save Entities
	if err := SaveEntities(s.SavePath, s.World); err != nil {
		fmt.Printf("Server Save Entities Error: %v\n", err)
	}
	// 3. Save Player (Authoritative)
	// Find the player entity to save
	s.World.entitiesMu.RLock()
	var player *PlayerEntity
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok {
			// For singleplayer/host, we pick the first player or specific one if we knew the name.
			// Currently we save to 'player.bin' which implies single player.
			player = p
			break
		}
	}
	s.World.entitiesMu.RUnlock()

	if player != nil {
		// Serialize Hotbar
		hotbarBytes := make([]byte, 9)
		for i := 0; i < 9; i++ {
			hotbarBytes[i] = byte(player.Inventory.Slots[i].ID)
		}

		fmt.Printf("Saving Player: %s at %.2f, %.2f, %.2f (Seed: %d)\n", player.UUID, player.X, player.Y, player.Z, s.World.seed)
		if err := SavePlayerState(s.SavePath, float32(player.X), float32(player.Y), float32(player.Z), player.SelectedSlot, hotbarBytes, s.World.seed); err != nil {
			fmt.Printf("Server Save Player Error: %v\n", err)
		}
	} else {
		// Fallback if no player entity found (e.g. just started server and quit)
		if err := SavePlayerState(s.SavePath, float32(s.InitialPosX), float32(s.InitialPosY), float32(s.InitialPosZ), 0, make([]byte, 9), s.World.seed); err != nil {
			fmt.Printf("Server Save Player Fallback Error: %v\n", err)
		}
	}
	fmt.Println("Server: World saved successfully.")
}

func (s *Server) Tick() {
	s.World.ProcessGenResults()
	s.processPendingChunks()
	s.UpdateEntities()

	// Garbage Collect Chunks
	// Radius 24 (generous buffer)
	// Only if single player or simple check
	// For robust MP: Check against ALL players.

	// Accessing s.Clients requires Lock.
	s.ClientsMu.RLock()
	var playerPositions [][2]int
	for _, c := range s.Clients {
		playerPositions = append(playerPositions, [2]int{c.LastChunkX, c.LastChunkZ})
	}
	s.ClientsMu.RUnlock()

	if len(playerPositions) > 0 {
		// Define unload callback to notify clients (Optional, but good for sync)
		onUnload := func(cx, cz int) {
			s.Broadcast(&PacketUnloadChunk{CX: int32(cx), CZ: int32(cz)})
			key := chunkKey{X: cx, Z: cz}

			// Crucial: Tell server to forget that clients know this chunk.
			// So next time they come close, we resend it.
			s.ClientsMu.RLock()
			for _, c := range s.Clients {
				delete(c.KnownChunks, key)
			}
			s.ClientsMu.RUnlock()
		}

		// Custom Unload Logic for Multiplayer support
		// We can't use World.UnloadChunks because it supports only one center.
		// We implement a custom loop here.

		unloadRadiusSq := 24 * 24

		s.World.chunksMu.Lock() // We need Lock to delete
		// Note: Iterating map with Lock is blocking, but necessary for delete.
		// To optimize: RLock first to find candidates, then Lock to delete?
		// Map iteration is safe with RLock? Yes.

		toRemove := make([]chunkKey, 0)
		for key := range s.World.chunks {
			// Check if chunk is far from ALL players
			keep := false
			for _, pos := range playerPositions {
				dx := key.X - pos[0]
				dz := key.Z - pos[1]
				if dx*dx+dz*dz <= unloadRadiusSq {
					keep = true
					break
				}
			}

			if !keep {
				// Also check if it's pending?
				// s.PendingChunks contains keys waiting for dispatch.
				// If we unload it, we might break pending logic.
				// PendingChunks keys allow us to keep duplicates?
				// Let's check s.PendingChunks (it's on Server struct, not World).
				// We should not unload if pending.
				// But PendingChunks map usage creates race if not locked?
				// PendingChunks is accessed in Tick (single thread? No, Tick is sequential).
				// Tick -> processPendingChunks. So safe from Tick.
				// But SendChunksAround (from Packet) writes it using ClientsMu? No, SendChunksAround writes PendingChunks.
				// SendChunksAround is called from HandlePacket (channel consumer in Start).
				// Tick is called from Ticker in Start.
				// They are SEQUENTIAL in the main select loop of Start!
				// So PendingChunks access is safe WITHOUT lock.

				isPending := false
				if _, ok := s.PendingChunks[key]; ok {
					isPending = true
				}

				if !isPending {
					toRemove = append(toRemove, key)
				}
			}
		}

		// Delete chunks
		for _, key := range toRemove {
			chunk := s.World.chunks[key]

			// SAVE BEFORE UNLOAD
			if chunk.dirty {
				if err := SaveChunk(s.SavePath, chunk, key.X, key.Z); err != nil {
					fmt.Printf("Error saving chunk %d,%d during unload: %v\n", key.X, key.Z, err)
				}
			}

			delete(s.World.chunks, key)

			// Safe Free (using our new thread-safe freeChunk)
			// We are holding chunksMu.Lock.
			// freeChunk acquires chunk.mu.Lock.
			// This is safe (Map -> Chunk order).
			// BUT: freeChunk calls w.chunkPool.Put(c).
			// IMPORTANT: We must unlock chunksMu before calling freeChunk?
			// The original UnloadChunks (in world_core.go) calls freeChunk INSIDE chunksMu.Lock loop?
			// Let's check step 309.
			// UnloadChunks: w.chunksMu.Lock(); ... w.freeChunk(chunk); ... w.chunksMu.Unlock().
			// Yes.
			// freeChunk: c.mu.Lock(); ... c.mu.Unlock().
			// So we hold Map Lock, take Chunk Lock.
			// Correct order.

			chunk.mu.Lock()
			s.World.chunkPool.Put(chunk)
			chunk.mu.Unlock()

			if onUnload != nil {
				onUnload(key.X, key.Z)
			}
		}
		s.World.chunksMu.Unlock()
	}
}

func (s *Server) SendInventory(player *PlayerEntity) {
	// Sync entire inventory to client
	// We MUST send empty slots (ID=0) too, otherwise client won't know items were consumed!
	for i, item := range player.Inventory.Slots {
		p := &PacketInventoryUpdate{
			SlotID: int32(i),
			ItemID: item.ID,
			Count:  item.Count,
		}
		s.BroadcastTo(player.UUID, p)
	}
}

func (s *Server) UpdateEntities() {
	s.World.TickEntities()

	// Handle Item Pickup & Remove dead entities
	s.World.entitiesMu.Lock()
	var toRemove []string

	// Collect Players for distance check
	var players []*PlayerEntity
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok {
			players = append(players, p)
		}
	}

	for _, e := range s.World.entities {
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
					rem := player.Inventory.Add(int32(item.ItemID), int32(item.Count))
					if rem < int32(item.Count) {
						// Some or all picked up
						// Sync Inventory
						s.SendInventory(player)

						if rem == 0 {
							item.Dead = true
						} else {
							item.Count = int(rem)
							// Update count for others
							meta := int32(item.ItemID) | (int32(item.Count) << 8)
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
				meta := int32(item.ItemID) | (int32(item.Count) << 8)
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

func (s *Server) StartTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.Listener = ln
	fmt.Printf("Server listening on %s\n", addr)

	go s.Start()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Check if this is a shutdown
			select {
			case <-s.Shutdown:
				// Silent exit on shutdown
				return nil
			default:
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
		}
		go s.handleNewConnection(conn)
	}
}

func (s *Server) handleNewConnection(conn net.Conn) {
	fmt.Printf("New connection from %s\n", conn.RemoteAddr())

	// We don't know the name yet, wait for login
	pkt, err := ReadPacket(conn)
	if err != nil {
		conn.Close()
		return
	}

	login, ok := pkt.(*PacketLogin)
	if !ok {
		conn.Close()
		return
	}

	name := login.Username
	s.ClientsMu.Lock()
	if old, exists := s.Clients[name]; exists {
		old.Conn.Close()
	}
	cc := &ClientConnection{
		Name:        name,
		Conn:        conn,
		Send:        make(chan Packet, 4096), // Increased buffer for chunk bursts
		KnownChunks: make(map[chunkKey]bool),
	}
	s.Clients[name] = cc
	s.ClientsMu.Unlock()

	// Start Writer Loop
	go func() {
		for p := range cc.Send {
			if err := WritePacket(cc.Conn, p); err != nil {
				fmt.Printf("Write error to %s: %v\n", name, err)
				break
			}
		}
		cc.Conn.Close()
	}()

	// Feed login to dispatcher
	s.PacketCh <- PacketWrapper{Packet: pkt, From: name}

	// Reader Loop
	for {
		p, err := ReadPacket(conn)
		if err != nil {
			fmt.Printf("Client %s disconnected: %v\n", name, err)
			break
		}
		s.PacketCh <- PacketWrapper{Packet: p, From: name}
	}

	s.ClientsMu.Lock()
	delete(s.Clients, name)
	s.ClientsMu.Unlock()
}

func (s *Server) Broadcast(p Packet) {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	for _, c := range s.Clients {
		// Non-blocking for global broadcast to prevent one laggy client from stalling server
		select {
		case c.Send <- p:
		default:
			// fmt.Println("Dropped broadcast packet to", c.Name)
		}
	}
}

func (s *Server) BroadcastTo(name string, p Packet) {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	if c, ok := s.Clients[name]; ok {
		// Blocking send for critical per-user data (Chunks)
		// With 4096 buffer, this should rarely block unless client is dead
		c.Send <- p
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
		meta = int32(item.ItemID) | (int32(item.Count) << 8)
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
}

func (s *Server) HandlePacket(wrap PacketWrapper) {
	pkt := wrap.Packet
	// client := s.Clients[wrap.From]

	switch p := pkt.(type) {
	case *PacketLogin:
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

		// 1. Send Initial Chunks (Radius 16)
		s.SendChunksAround(p.Username, 0, 0, 16)

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

		s.ClientsMu.RLock()
		client, ok := s.Clients[p.Username]
		s.ClientsMu.RUnlock()

		if ok {
			client.Send <- &PacketSpawnPoint{
				X: spawnX,
				Y: spawnY,
				Z: spawnZ,
			}
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
				meta = int32(item.ItemID) | (int32(item.Count) << 8)
			}

			s.ClientsMu.RLock()
			client, ok := s.Clients[p.Username]
			s.ClientsMu.RUnlock()
			if ok {
				client.Send <- &PacketEntitySpawn{
					EntityID: e.GetUUID(),
					Type:     e.GetType(),
					X:        ex,
					Y:        ey,
					Z:        ez,
					Yaw:      eyaw,
					Pitch:    epitch,
					Metadata: meta,
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
				BaseEntity: BaseEntity{
					UUID: p.Username, Type: EntityPlayer,
					X: spawnX, Y: spawnY, Z: spawnZ,
				},
			}
			s.SpawnEntity(playerEnt)
		} else {
			fmt.Printf("Player %s rejoining existing entity\n", p.Username)
		}

	case *PacketGameMode:
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
			player.GameMode = p.Mode
			fmt.Printf("Player %s switched to GameMode %d\n", wrap.From, p.Mode)
			// Echo back to confirm (or broadcast if others need to know)
			s.BroadcastTo(wrap.From, p)
		}

	case *PacketPlayerMove:
		// Update player position in ServerWorld for saving
		s.InitialPosX = p.X
		s.InitialPosY = p.Y
		s.InitialPosZ = p.Z
		s.HasSavedPos = true

		// Update Player Entity in world for broadcasting
		s.World.entitiesMu.RLock()
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				e.SetPosition(p.X, p.Y, p.Z)
				e.SetRotation(p.Yaw, p.Pitch)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

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
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				player = e.(*PlayerEntity)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player == nil {
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
						s.BroadcastTo(wrap.From, &PacketBlockChange{X: p.X, Y: p.Y, Z: p.Z, BlockID: blockAir})
						return
					}
				}
				// Sync Inventory
				s.SendInventory(player)
			}
		}

		// Apply block change to World
		// Validation logic would go here
		fmt.Printf("Server: Block set at %d %d %d\n", p.X, p.Y, p.Z)
		s.World.SetBlockAt(int(p.X), int(p.Y), int(p.Z), p.BlockID)

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
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				player = e.(*PlayerEntity)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player != nil {
			// Handle Cursor Update (Slot -1)
			if p.SlotID == -1 {
				// Only allow arbitrary cursor setting in Creative Mode?
				// For now let's allow it generally or check GameMode if strict.
				// Since we use this for robust sync, let's allow it.
				player.CursorItem = Item{ID: p.ItemID, Count: p.Count}
			} else if p.SlotID >= 0 && p.SlotID < 36 {
				player.Inventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count}
				// fmt.Printf("Server: Updated slot %d for %s to %d:%d\n", p.SlotID, wrap.From, p.ItemID, p.Count)
			}
		}

	case *PacketSlotChange:
		s.World.entitiesMu.RLock()
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				player = e.(*PlayerEntity)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player != nil {
			if p.Slot >= 0 && p.Slot < 9 {
				player.SelectedSlot = int(p.Slot)
			}
		}

	case *PacketClickWindow:
		s.World.entitiesMu.RLock()
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				player = e.(*PlayerEntity)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player != nil {
			if p.IsCreative {
				// Creative Pick (Client authority for palette acts as "spawn item")
				// We trust the client is picking a valid block from palette.
				// Put it in Cursor? Or directly in Slot?
				// Usually Creative Pick puts directly in Slot (handled by InventoryUpdate for legacy)
				// But let's support "Pick to Cursor" -> "Place in Slot".
				// For now, let's assume Creative Palette pickup sends specific slot update.
				// If p.SlotID is -1 (outside), maybe it's dropping?
				// Let's implement basic Creative Cursor Set:
				// If clicking inside inventory with creative flag, maybe just Fill Stack?
				// For simplicity, let's defer Creative logic to client sending SetSlot,
				// OR if we want server authoritative, we need to know what they clicked in the palette.
				// Since palette is static, we could validate.
				// BUT, user's request is about Survival logic mostly.
			} else {
				// Survival Logic
				// Slot -999 is usually "Drop Outside" in MC, but we can stick to 0-35 for now.
				if p.SlotID >= 0 && p.SlotID < 36 {
					slot := &player.Inventory.Slots[p.SlotID]
					cursor := &player.CursorItem

					// Logic mirroring input.go
					if p.Button == 0 { // Left Click
						if cursor.ID == 0 {
							// Pickup / Swap
							if slot.ID != 0 {
								// Pickup
								*cursor = *slot
								*slot = Item{}
							}
						} else {
							// Place / Swap / Stack
							if slot.ID == 0 {
								// Place All
								*slot = *cursor
								*cursor = Item{}
							} else if slot.ID == cursor.ID {
								// Stack
								space := int32(64) - slot.Count
								if space > 0 {
									toAdd := cursor.Count
									if toAdd > space {
										toAdd = space
									}
									slot.Count += toAdd
									cursor.Count -= toAdd
									if cursor.Count == 0 {
										cursor.ID = 0
									}
								}
							} else {
								// Swap
								temp := *slot
								*slot = *cursor
								*cursor = temp
							}
						}
					} else if p.Button == 1 { // Right Click
						if cursor.ID == 0 {
							if slot.ID != 0 {
								// Split (Take Half)
								half := slot.Count / 2
								rem := slot.Count - half
								if half > 0 {
									cursor.ID = slot.ID
									cursor.Count = half
									slot.Count = rem
									if slot.Count == 0 {
										slot.ID = 0
									}
								}
							}
						} else {
							// Place One
							if slot.ID == 0 {
								slot.ID = cursor.ID
								slot.Count = 1
								cursor.Count--
							} else if slot.ID == cursor.ID {
								if slot.Count < 64 {
									slot.Count++
									cursor.Count--
								}
							}
							if cursor.Count == 0 {
								cursor.ID = 0
							}
						}
					}

					// Send Updates
					// 1. Update Clicked Slot
					s.SendTo(wrap.From, &PacketInventoryUpdate{
						SlotID: p.SlotID,
						ItemID: slot.ID,
						Count:  slot.Count,
					})
					// 2. Update Cursor (Slot -1? Or special packet?)
					// MC uses SetSlot -1 for cursor.
					// We need to support SlotID -1 in InventoryUpdate or add PacketSetCursor.
					// Let's reuse InventoryUpdate with SlotID -1 for Cursor.
					s.SendTo(wrap.From, &PacketInventoryUpdate{
						SlotID: -1,
						ItemID: cursor.ID,
						Count:  cursor.Count,
					})
				}
			}
		}

	case *PacketPlayerAction:
		// 1. Find player pos/rot
		s.World.entitiesMu.RLock()
		var px, py, pz float64
		var yaw, pitch float32
		found := false
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				px, py, pz = e.GetPosition()
				px, py, pz = e.GetPosition()
				yaw, pitch = e.GetRotation()
				found = true
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if !found {
			return
		}

		// 2. Spawn Item
		itemID := byte(p.Value & 0xFF)
		count := int((p.Value >> 8) & 0xFF)
		if count <= 0 {
			count = 1
		}

		s.World.entitiesMu.RLock()
		// Re-fetch player safely (reuse finding logic or just grab from loop above which we already did)
		// We found 'player' in s.World.entities earlier but didn't cast/store it as *PlayerEntity cleanly.
		// Let's optimize: reuse the 'found' logic to get the PlayerEntity object properly.
		var player *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == wrap.From {
				player = e.(*PlayerEntity)
				break
			}
		}
		s.World.entitiesMu.RUnlock()

		if player != nil && player.GameMode == ModeSurvival {
			if !player.Inventory.Consume(int32(itemID), int32(count)) {
				// Failed to consume, cannot drop
				fmt.Printf("Player %s tried to drop Item %d without valid count.\n", wrap.From, itemID)
				s.SendInventory(player) // Sync back to fix client state
				return
			}
			s.SendInventory(player) // Sync successful removal
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
			ItemID:      itemID,
			Count:       count,
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
		maxRange := 18 // Render distance + buffer

		if dx*dx+dz*dz > maxRange*maxRange {
			// Request is too far, ignore (anti-cheat)
			return
		}

		// Get or create chunk
		chunk := s.World.ensureChunk(cx, cz)
		if chunk == nil {
			return
		}

		// If chunk not generated, try loading from disk, then generate
		if !chunk.generated {
			chunk.mu.Lock()
			// Double-check inside lock (another goroutine might have loaded it)
			if !chunk.generated {
				// Try loading from saved data first
				if TryLoadChunk(s.SavePath, chunk, cx, cz) {
					// Successfully loaded from disk
					ensureChunkSections(chunk)
					chunk.rebuildHeightMap()
					chunk.rebuildTorchCount()
				} else {
					// No saved data, generate terrain
					generateChunkData(s.World.seed, cx, cz, chunk)
					ensureChunkSections(chunk)
				}
				chunk.generated = true
			}
			chunk.mu.Unlock()
		}

		// Serialize and send
		data := make([]byte, chunkWidth*chunkHeight*chunkWidth)
		light := make([]byte, chunkWidth*chunkHeight*chunkWidth)

		chunk.mu.RLock()
		idx := 0
		for lx := 0; lx < chunkWidth; lx++ {
			for y := 0; y < chunkHeight; y++ {
				for lz := 0; lz < chunkWidth; lz++ {
					data[idx] = chunk.blocks[lx][y][lz]
					light[idx] = (chunk.skyLight[lx][y][lz] << 4) | (chunk.blockLight[lx][y][lz] & 0x0F)
					idx++
				}
			}
		}
		chunk.mu.RUnlock()

		client.Send <- &PacketChunkData{
			CX:        int32(cx),
			CZ:        int32(cz),
			Data:      data,
			LightData: light,
		}
	} // End of switch
} // End of HandlePacket

func (s *Server) SendChunksAround(username string, centerCX, centerCZ, radius int) {
	s.ClientsMu.RLock()
	client, ok := s.Clients[username]
	s.ClientsMu.RUnlock()
	if !ok {
		return
	}

	// Identify missing chunks
	missing := []chunkKey{}
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			cx, cz := centerCX+dx, centerCZ+dz
			key := chunkKey{X: cx, Z: cz}
			if !client.KnownChunks[key] {
				client.KnownChunks[key] = true
				missing = append(missing, key)
			}
		}
	}

	if len(missing) == 0 {
		return
	}

	// Sort missing chunks by distance to player (Spiral/Center-out)
	sort.Slice(missing, func(i, j int) bool {
		di := (missing[i].X-centerCX)*(missing[i].X-centerCX) + (missing[i].Z-centerCZ)*(missing[i].Z-centerCZ)
		dj := (missing[j].X-centerCX)*(missing[j].X-centerCX) + (missing[j].Z-centerCZ)*(missing[j].Z-centerCZ)
		return di < dj
	})

	for _, key := range missing {
		// Trigger generation priority
		s.World.requestChunk(key.X, key.Z)

		// Add to pending
		if list, ok := s.PendingChunks[key]; ok {
			// Check if already in list to avoid dups
			found := false
			for _, n := range list {
				if n == username {
					found = true
					break
				}
			}
			if !found {
				s.PendingChunks[key] = append(list, username)
			}
		} else {
			s.PendingChunks[key] = []string{username}
		}
	}
}

func (s *Server) processPendingChunks() {
	if len(s.PendingChunks) == 0 {
		return
	}

	// Iterate keys (random order is fine-ish, generation order matters more)
	// We could optimize to process only N per tick if needed

	toRemove := []chunkKey{}

	for key, users := range s.PendingChunks {
		chunk := s.World.getChunkIfGenerated(key.X, key.Z)
		if chunk == nil {
			continue
		}

		// Chunk is ready! Serialize it.
		data := make([]byte, chunkWidth*chunkHeight*chunkWidth)
		light := make([]byte, chunkWidth*chunkHeight*chunkWidth)

		chunk.mu.RLock()
		idx := 0
		for lx := 0; lx < chunkWidth; lx++ {
			for y := 0; y < chunkHeight; y++ {
				for lz := 0; lz < chunkWidth; lz++ {
					data[idx] = chunk.blocks[lx][y][lz]
					// Combine sky (high 4 bits) and block (low 4 bits)
					light[idx] = (chunk.skyLight[lx][y][lz] << 4) | (chunk.blockLight[lx][y][lz] & 0x0F)
					idx++
				}
			}
		}
		chunk.mu.RUnlock()

		p := &PacketChunkData{
			CX:        int32(key.X),
			CZ:        int32(key.Z),
			Data:      data,
			LightData: light,
		}

		// Send to all waiting users
		for _, user := range users {
			s.BroadcastTo(user, p)
		}

		toRemove = append(toRemove, key)
	}

	for _, key := range toRemove {
		delete(s.PendingChunks, key)
	}
}

func (s *Server) SendTo(player string, p Packet) {
	s.ClientsMu.RLock()
	client, ok := s.Clients[player]
	s.ClientsMu.RUnlock()
	if ok {
		client.Send <- p
	}
}

func (s *Server) handleCommand(player string, cmd string) {
	// Simple command parser
	parts := make([]string, 0)
	current := ""
	for _, c := range cmd {
		if c == ' ' {
			if len(current) > 0 {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if len(current) > 0 {
		parts = append(parts, current)
	}

	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/tp":
		if len(parts) < 4 {
			s.SendTo(player, &PacketChat{Message: "Usage: /tp x y z"})
			return
		}
		var x, y, z float64
		fmt.Sscanf(parts[1], "%f", &x)
		fmt.Sscanf(parts[2], "%f", &y)
		fmt.Sscanf(parts[3], "%f", &z)

		s.SendTo(player, &PacketPlayerMove{X: x, Y: y, Z: z, Yaw: 0, Pitch: 0})
		s.SendTo(player, &PacketChat{Message: fmt.Sprintf("Teleported to %.1f %.1f %.1f", x, y, z)})

	case "/give":
		if len(parts) < 2 {
			s.SendTo(player, &PacketChat{Message: "Usage: /give id [count]"})
			return
		}
		var id int
		count := 64
		fmt.Sscanf(parts[1], "%d", &id)
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &count)
		}

		s.World.entitiesMu.Lock()
		var pEnt *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == player {
				if pe, ok := e.(*PlayerEntity); ok {
					pEnt = pe
				}
				break
			}
		}

		if pEnt != nil {
			remaining := int32(count)
			itemID := int32(id)

			// 1. Try to stack
			for i := 0; i < 36 && remaining > 0; i++ {
				slot := &pEnt.Inventory.Slots[i]
				if slot.ID == itemID && slot.Count < 64 {
					space := int32(64) - slot.Count
					toAdd := remaining
					if toAdd > space {
						toAdd = space
					}
					slot.Count += toAdd
					remaining -= toAdd
					// Send Update
					s.SendTo(player, &PacketInventoryUpdate{SlotID: int32(i), ItemID: slot.ID, Count: slot.Count})
				}
			}

			// 2. Fill empty slots
			for i := 0; i < 36 && remaining > 0; i++ {
				slot := &pEnt.Inventory.Slots[i]
				if slot.ID == 0 { // Empty
					toAdd := remaining
					if toAdd > 64 {
						toAdd = 64
					}
					slot.ID = itemID
					slot.Count = toAdd
					remaining -= toAdd
					// Send Update
					s.SendTo(player, &PacketInventoryUpdate{SlotID: int32(i), ItemID: slot.ID, Count: slot.Count})
				}
			}

			if remaining < int32(count) {
				s.SendTo(player, &PacketChat{Message: fmt.Sprintf("Given %d of block %d", int32(count)-remaining, id)})
			} else {
				s.SendTo(player, &PacketChat{Message: "Inventory full"})
			}
		} else {
			s.SendTo(player, &PacketChat{Message: "Player entity not found"})
		}
		s.World.entitiesMu.Unlock()

	default:
		s.SendTo(player, &PacketChat{Message: "Unknown command: " + parts[0]})
	}
}
