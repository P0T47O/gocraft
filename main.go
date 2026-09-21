package main

import (
	"flag"
	"fmt"
	"time"
)

// ProgramState
type ProgramState int

const (
	StateMenu ProgramState = iota
	StatePlaying
)

// MenuState
type MenuPage int

const (
	MenuMain MenuPage = iota
	MenuSingleplayer
	MenuMultiplayer
	MenuCreateWorld
	MenuSettings
)

var (
	chunkPool            *ChunkPool
	perfMon              *PerformanceMonitor
	remoteEntities       = make(map[string]*RemoteEntity)
	pendingChunkRequests = make(map[chunkKey]time.Time) // Retry lost/rejected requests.

	// CLI Flags
	isServer   = flag.Bool("server", false, "Start as dedicated server")
	serverAddr = flag.String("addr", "127.0.0.1:25565", "Server address to listen/connect")
	username   = flag.String("name", "Player"+fmt.Sprint(time.Now().Unix()%1000), "Username")
	useWebGPU  = flag.Bool("webgpu", true, "Use native WebGPU (Windows); false selects the legacy reference renderer")

	// Game State
	currentState ProgramState = StateMenu
	menuPage     MenuPage     = MenuMain

	// Game Resources (Initialized when playing)
	world  *World
	client *Client
	server *Server
	input  *InputState
	assets *RenderAssets
	camera gameCamera

	// Menu Resources
	ui           *UIComponents
	saveList     []SaveInfo
	ipInput      string = "127.0.0.1:25565"
	newWorldName string = "New World"
	newWorldSeed string = "12345"
	isPaused     bool   = false

	// Client Game State
	localInventory  Inventory
	currentGameMode byte = ModeSurvival

	// Chat
	isChatOpen  bool   = false
	chatInput   string = ""
	chatHistory []string
)

var mobsRenderer *MobRenderer

type RemoteEntity struct {
	MobKind, MobState                                   string
	TargetYaw, AnimPhase, AnimBlend, MobHurt, DeathTime float32
	MobHealth                                           int
	ID                                                  string
	Type                                                EntityType
	X, Y, Z                                             float64 // Visual position (Lerped)
	TX, TY, TZ                                          float64 // Target position
	Yaw, Pitch                                          float32
	Metadata                                            int32
}

func main() {
	flag.Parse()
	if *mobPreview {
		runMobPreview()
		return
	}
	initBlockRegistry()
	if err := loadRuntimeMobs(); err != nil {
		panic(err)
	}

	// Directed Server Mode
	if *isServer {
		server := NewServer(RootSaveDir + "/Dedicated")
		if err := server.StartTCP(*serverAddr); err != nil {
			fmt.Printf("Failed to start server: %v\n", err)
		}
		return
	}

	// Load Settings
	settings := LoadSettings()
	if *useWebGPU {
		if err := runNativeWebGPU(settings); err != nil {
			fmt.Printf("Native WebGPU failed: %v\n", err)
		}
		return
	}
	runLegacyRaylib(settings)
}
