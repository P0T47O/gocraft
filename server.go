package main

import (
	"fmt"
	"math"
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

	// Initial player state (Loaded from save)
	InitialPosX, InitialPosY, InitialPosZ float64
	HasSavedPos                           bool
	LastSentPos                           map[string][3]float64
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

func NewServer() *Server {
	world := NewFlatWorld()
	// Authoritative Load (MUST BE BEFORE STARTING WORKERS)
	hasPos, px, py, pz, _ := LoadWorld(world)

	// Now we can start the workers with the correct seed
	fmt.Printf("Server: Authoritative Seed Loaded: %d\n", world.seed)
	world.StartBackend()

	s := &Server{
		World:       world,
		Clients:     make(map[string]*ClientConnection),
		PacketCh:    make(chan PacketWrapper, 1024),
		Shutdown:    make(chan bool),
		InitialPosX: px,
		InitialPosY: py,
		InitialPosZ: pz,
		HasSavedPos: hasPos,
		LastSentPos: make(map[string][3]float64),
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

func (s *Server) Start() {
	fmt.Println("Server starting...")
	ticker := time.NewTicker(50 * time.Millisecond) // 20 TPS

	for {
		select {
		case <-s.Shutdown:
			s.Save()
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
	// 1. Save Chunks
	if err := SaveWorldChunks(s.World); err != nil {
		fmt.Printf("Server Save Chunks Error: %v\n", err)
	}
	// 2. Save Entities
	if err := SaveEntities(s.World); err != nil {
		fmt.Printf("Server Save Entities Error: %v\n", err)
	}
	// 3. Save Player (Authoritative local player if exists)
	// In integrated server, we use the local client's position if possible
	// For now, we rely on the last PacketPlayerMove update in s.InitialPos
	// Actually we should capture the last known position of the local client
	if err := SavePlayerState(float32(s.InitialPosX), float32(s.InitialPosY), float32(s.InitialPosZ), 0, make([]byte, 9), s.World.seed); err != nil {
		fmt.Printf("Server Save Player Error: %v\n", err)
	}
	fmt.Println("Server: World saved successfully.")
}

func (s *Server) Tick() {
	s.World.ProcessGenResults()
	s.UpdateEntities()
}

func (s *Server) UpdateEntities() {
	s.World.TickEntities()

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
			e.ClearDirty()
		}
	}
}

func (s *Server) StartTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("Server listening on %s\n", addr)

	go s.Start()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Accept error: %v\n", err)
			continue
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
		Send:        make(chan Packet, 256),
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
		select {
		case c.Send <- p:
		default:
		}
	}
}

func (s *Server) BroadcastTo(name string, p Packet) {
	s.ClientsMu.RLock()
	c, ok := s.Clients[name]
	s.ClientsMu.RUnlock()
	if ok {
		select {
		case c.Send <- p:
		default:
		}
	}
}

func (s *Server) SpawnEntity(e Entity) {
	s.World.entitiesMu.Lock()
	s.World.entities = append(s.World.entities, e)
	s.World.entitiesMu.Unlock()

	x, y, z := e.GetPosition()
	yaw, pitch := e.GetRotation()
	s.Broadcast(&PacketEntitySpawn{
		EntityID: e.GetUUID(),
		Type:     e.GetType(),
		X:        x,
		Y:        y,
		Z:        z,
		Yaw:      yaw,
		Pitch:    pitch,
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
		if _, ok := s.Clients[p.Username]; !ok {
			conn := &ClientConnection{
				Name:        p.Username,
				Send:        make(chan Packet, 128),
				KnownChunks: make(map[chunkKey]bool),
			}
			s.Clients[p.Username] = conn
		}

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

		s.Clients[p.Username].Send <- &PacketSpawnPoint{
			X: spawnX,
			Y: spawnY,
			Z: spawnZ,
		}

		// 3. Send Existing Entities
		s.World.entitiesMu.RLock()
		for _, e := range s.World.entities {
			if e.GetUUID() == p.Username {
				continue
			} // Skip self if represented as entity
			ex, ey, ez := e.GetPosition()
			eyaw, epitch := e.GetRotation()
			s.Clients[p.Username].Send <- &PacketEntitySpawn{
				EntityID: e.GetUUID(),
				Type:     e.GetType(),
				X:        ex,
				Y:        ey,
				Z:        ez,
				Yaw:      eyaw,
				Pitch:    epitch,
			}
		}
		s.World.entitiesMu.RUnlock()

		// 4. Register new player as entity and broadcast to others
		playerEnt := &PlayerEntity{
			BaseEntity: BaseEntity{
				UUID: p.Username, Type: EntityPlayer,
				X: spawnX, Y: spawnY, Z: spawnZ,
			},
		}
		s.SpawnEntity(playerEnt)

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

	case *PacketBlockChange:
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
	}
}

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

	go func() {
		for i, key := range missing {
			cx, cz := key.X, key.Z
			// Trigger generation
			s.World.requestChunk(cx, cz)

			// Wait for generation
			var chunk *Chunk
			for {
				chunk = s.World.getChunkIfGenerated(cx, cz)
				if chunk != nil {
					break
				}
				time.Sleep(5 * time.Millisecond) // Faster polling for priority chunks
			}

			// Serialize (Read Lock)
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
				CX:        int32(cx),
				CZ:        int32(cz),
				Data:      data,
				LightData: light,
			}
			s.BroadcastTo(username, p)

			// Burst Mode: First 64 chunks (radius ~4) send with zero delay to fill proximity FAST.
			// Then switch to small delay to prevent client network/CPU spikes.
			if i > 64 {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
}
