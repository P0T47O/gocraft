package main

import (
	"flag"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"gocraft/platform"
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
	useWebGPU  = flag.Bool("webgpu", false, "Use the experimental WebGPU world renderer (Windows only)")

	// Game State
	currentState ProgramState = StateMenu
	menuPage     MenuPage     = MenuMain

	// Game Resources (Initialized when playing)
	world  *World
	client *Client
	server *Server
	input  *InputState
	assets *RenderAssets
	camera rl.Camera3D

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
	rl.SetTraceLogLevel(rl.LogWarning) // Suppress INFO logs (texture, etc.)
	rl.InitWindow(int32(settings.ResolutionWidth), int32(settings.ResolutionHeight), "GoCraft")
	rl.SetExitKey(0) // Disable default ESC exit to allow custom Pause Menu
	defer rl.CloseWindow()

	var mobErr error
	mobsRenderer, mobErr = newMobRenderer(mobContent)
	if mobErr != nil {
		fmt.Printf("Mob visuals unavailable; using placeholder: %v\n", mobErr)
	}
	defer mobsRenderer.Close()
	platform.InitGLOnce()
	rl.SetTargetFPS(60)

	// Initialize block definitions (CRITICAL: must be called before any world/rendering)
	initBlockRegistry()
	InitRecipes()

	// Resources
	ui = NewUIComponents()
	perfMon = NewPerformanceMonitor()
	defer perfMon.Close()

	if *username == "" || (len(*username) >= 6 && (*username)[:6] == "Player") { // Heuristic: If default or generic, use settings
		if settings.PlayerName != "" {
			*username = settings.PlayerName
		} else {
			// If settings is empty, set it to the random one and save
			// Actually, settings default is "Player" now.
			if settings.PlayerName == "Player" {
				// Keep the random suffix if it's just "Player" to avoid collisions?
				// Or just let user change it.
			}
			*username = settings.PlayerName
		}
	}

	const webGPUFrameInterval = time.Second / 60
	for !rl.WindowShouldClose() && !quitRequested {
		// The WebGPU path owns presentation while playing. Raylib continues to own
		// the native window and input, but Begin/EndDrawing must not swap the same
		// HWND while WebGPU is presenting to it.
		if *useWebGPU && currentState == StatePlaying {
			if err := ensureExperimentalWebGPURenderer(); err != nil {
				fmt.Printf("WebGPU renderer unavailable, falling back to OpenGL: %v\n", err)
				*useWebGPU = false
				continue
			}

			// Raylib normally enforces SetTargetFPS from EndDrawing. The WebGPU path
			// intentionally skips EndDrawing to avoid an OpenGL swap on the same HWND,
			// so pace this loop explicitly. Without this guard, the game update runs as
			// fast as the CPU allows while rl.GetFrameTime still reflects the last GL
			// frame, causing extreme movement speed and flooding reliable network queues.
			frameStarted := time.Now()
			rl.PollInputEvents()
			updateGame()
			if currentState == StatePlaying {
				if err := drawExperimentalWebGPUFrame(); err != nil {
					fmt.Printf("WebGPU frame failed: %v\n", err)
					quitRequested = true
				}
			}
			perfMon.Update()
			if remaining := webGPUFrameInterval - time.Since(frameStarted); remaining > 0 {
				time.Sleep(remaining)
			}
			continue
		}

		rl.BeginDrawing()
		rl.ClearBackground(invBackground)

		switch currentState {
		case StateMenu:
			updateMenu()
			drawMenu()
		case StatePlaying:
			updateGame()
			if currentState == StatePlaying {
				drawGame()
			}
		}

		rl.EndDrawing()
		perfMon.Update()
	}

	// Cleanup on Exit
	if currentState == StatePlaying {
		exitGame()
	}
}

func updateMenu() {
	rl.ShowCursor()
	if rl.IsKeyPressed(rl.KeyEscape) && menuPage != MenuMain {
		if menuPage == MenuSettings {
			SaveSettings()
			*username = LoadSettings().PlayerName
		}
		if menuPage == MenuCreateWorld {
			menuNavigate(MenuSingleplayer)
		} else {
			menuNavigate(MenuMain)
		}
	}

	// Simple Menu Logic
	switch menuPage {
	case MenuMain:
		// Logic handles in Draw for IMGUI simplicity
	case MenuSingleplayer:
		// Refresh save list occasionally?
	case MenuCreateWorld:
	case MenuMultiplayer:
	}
}
