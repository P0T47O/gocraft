package main

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// chunkDataBufPool reuses large byte slices for chunk serialization to avoid
// allocating 2 × 64KB per chunk per tick in processPendingChunks.
var chunkDataBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, chunkWidth*chunkHeight*chunkWidth)
	},
}

type Server struct {
	MobSpawnTicks     int
	Containers        map[BlockPos]*BlockContainer
	ContainerSessions map[string]ContainerSession
	ContainerToken    int32
	World             *World // The authoritative world
	Paused            atomic.Bool
	Clients           map[string]*ClientConnection
	ClientsMu         sync.RWMutex
	PacketCh          chan PacketWrapper
	Shutdown          chan bool
	Done              chan bool
	stopOnce          sync.Once
	networkWG         sync.WaitGroup
	connections       map[net.Conn]bool // Includes sockets still waiting for login; ClientsMu protects it.

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
	done        chan struct{}
	closeOnce   sync.Once
	Name        string
	Conn        net.Conn
	Send        chan Packet // Buffer for outgoing packets
	KnownChunks map[chunkKey]bool
	LastChunkX  int
	LastChunkZ  int
}

type PacketWrapper struct {
	Packet     Packet
	From       string
	Connection *ClientConnection
}

func NewServer(savePath string) *Server {
	world := NewFlatWorld()
	// Authoritative Load (MUST BE BEFORE STARTING WORKERS)
	hasPos, px, py, pz, loadErr := LoadWorld(savePath, world)
	if loadErr != nil {
		panic(fmt.Errorf("load world entities: %w", loadErr))
	}

	// Set save path for chunk loading in background workers
	world.SavePath = savePath

	if err := loadSurvivalPlayers(savePath, world); err != nil {
		panic(fmt.Errorf("load player inventory: %w", err))
	}

	// Now we can start the workers with the correct seed
	fmt.Printf("Server: Authoritative Seed Loaded: %d\n", world.seed)
	world.StartBackend()
	InitRecipes()

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
		connections:   make(map[net.Conn]bool),
	}

	if err := s.loadContainers(); err != nil {
		panic(err)
	}
	if !hasPos && len(world.entities) == 0 {
		// Spawn a starter pig
		p := newMob("pig", "Piggy-01", 8, float64(world.HeightAt(8, 20))+.501, 20)
		s.SpawnEntity(p)
	}

	return s
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() { close(s.Shutdown) })
}

func (s *Server) Start() {
	fmt.Println("Server starting...")
	ticker := time.NewTicker(50 * time.Millisecond) // 20 TPS
	defer ticker.Stop()
	playerSaveTicker := time.NewTicker(60 * time.Second)
	defer playerSaveTicker.Stop()

	for {
		select {
		case <-s.Shutdown:
			if s.Listener != nil {
				s.Listener.Close()
			}
			s.ClientsMu.Lock()
			for conn := range s.connections {
				conn.Close()
			}
			for _, c := range s.Clients {
				c.close()
			}
			s.ClientsMu.Unlock()
			s.networkWG.Wait()
			s.Save()
			s.World.Close()
			close(s.Done)
			return
		case <-playerSaveTicker.C:
			s.Save()
		case <-ticker.C:
			s.Tick()
		case wrap := <-s.PacketCh:
			s.HandlePacket(wrap)
		}
	}
}
