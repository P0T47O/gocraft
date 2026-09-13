package main

import (
	"flag"
	"fmt"
	"gocraft/platform"
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
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

	for !rl.WindowShouldClose() && !quitRequested {
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

func startGame(savePath string, ip string, isMultiplayer bool) {
	fmt.Println("Starting Game...")

	// Load Common Assets
	// chunkPool = NewChunkPool(1024) // Share the global pool? Or new? Global is fine.
	// initBlockRegistry() // Already initialized in main()
	assets = loadRenderAssets()

	// Initialize Camera
	camera = rl.Camera3D{
		Position:   rl.NewVector3(8, 8, 20),
		Target:     rl.NewVector3(8, 2, 8),
		Up:         rl.NewVector3(0, 1, 0),
		Fovy:       70,
		Projection: rl.CameraPerspective,
	}

	var err error
	if !isMultiplayer {
		// Singleplayer: Start Internal Server
		fmt.Printf("Starting Internal Server at %s\n", savePath)
		server = NewServer(savePath)
		if err = server.ListenTCP("127.0.0.1:0"); err != nil {
			fmt.Printf("Cannot start local server: %v\n", err)
			server.World.Close()
			server = nil
			assets.unload()
			assets = nil
			return
		}
		ip = server.Listener.Addr().String()
		go server.ServeTCP()
	}

	fmt.Printf("Connecting to %s...\n", ip)
	client, err = ConnectTCP(ip, *username)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		menuError = "Connection failed: " + err.Error()
		exitGame()
		if isMultiplayer {
			menuPage = MenuMultiplayer
		}
		return
	}

	// Client World
	clear(pendingChunkRequests)
	clear(remoteEntities)
	world = NewClientWorld()
	world.StartMeshWorkers(assets, 8)

	// Input
	input = NewInputState()
	input.InitFromCamera(camera)

	rl.DisableCursor()
	isPaused = false
	pauseSettings = false
	currentState = StatePlaying
}

func exitGame() {
	if client != nil {
		client.Close()
		client = nil
	}
	if server != nil {
		server.Stop()
		// Wait for server to finish saving
		select {
		case <-server.Done:
		case <-time.After(5 * time.Second): // Fail-safe
			fmt.Println("Server shutdown timed out")
		}
		server = nil
	}
	if world != nil {
		world.Close()
		world = nil
	}
	if assets != nil {
		assets.unload()
		assets = nil
	}
	clear(pendingChunkRequests)
	clear(remoteEntities)
	// Reset State
	currentState = StateMenu
	menuPage = MenuMain
	isPaused = false
	pauseSettings = false
	rl.EnableCursor()
}

func updateGame() {
	if isPaused && pauseSettings && rl.IsKeyPressed(rl.KeyEscape) {
		SaveSettings()
		pauseSettings = false
		ui.ActiveID = ""
		return
	}
	if !input.isDead() {
		// Chat Input
		if isChatOpen {
			// Handle keys
			char := rl.GetCharPressed()
			for char > 0 {
				if char >= 32 && char <= 125 {
					chatInput += string(char)
				}
				char = rl.GetCharPressed()
			}

			if rl.IsKeyPressed(rl.KeyBackspace) {
				if len(chatInput) > 0 {
					chatInput = chatInput[:len(chatInput)-1]
				}
			}

			if rl.IsKeyPressed(rl.KeyEnter) {
				if len(chatInput) > 0 {
					client.Send(&PacketChat{Message: chatInput})
					chatInput = ""
				}
				isChatOpen = false
				rl.DisableCursor()
			}

			if rl.IsKeyPressed(rl.KeyEscape) {
				isChatOpen = false
				rl.DisableCursor()
			}

			return // Block other inputs while chat is open
		} else if !isPaused && !input.InventoryOpen && rl.IsKeyPressed(rl.KeyEnter) {
			isChatOpen = true
			rl.EnableCursor()
			// rl.SetMousePosition? No, let cursor be free.
			return
		}

		if rl.IsKeyPressed(rl.KeyEscape) {
			if input.InventoryOpen {
				input.closeContainerUI()
				input.InventoryOpen = false
				input.CraftingStation = 0
				input.SkipCamera = true
				if !isPaused {
					rl.DisableCursor()
				}
			} else {
				isPaused = !isPaused
				if server != nil {
					server.Paused.Store(isPaused)
				}
				if isPaused {
					rl.EnableCursor()
					ui.ActiveID = ""
				} else {
					rl.DisableCursor()
					// Reset mouse to center to prevent view jump
					rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
				}
			}
		}

	}

	// Inventory toggle is handled in HandleInput -> ToggleInventory

	// Singleplayer Pause: Freeze update loop
	if isPaused && server != nil {
		return
	}

	dt := rl.GetFrameTime()
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
		input.updateRespawnRequest(client)
		requestMissingChunks()
		return
	}
	if input.AwaitingTerrain {
		cx := divFloor(blockIndexFromCoord(camera.Position.X), chunkWidth)
		cz := divFloor(blockIndexFromCoord(camera.Position.Z), chunkWidth)
		if world.getChunkIfGenerated(cx, cz) == nil {
			requestMissingChunks()
			return
		}
		input.AwaitingTerrain = false
	}
	if !input.VitalsReady {
		requestMissingChunks()
		return
	}
	input.HurtFlash = max(float32(0), input.HurtFlash-dt)

	HandleInput(world, &camera, input, client)
	world.ProcessImmediateMeshes(assets, 16)
	clear(world.lightChanged) // Only the server publishes authoritative light updates.

	// Client-Pull: Request any missing chunks
	requestMissingChunks()

	client.Update(&camera, input)

	updateEntities(dt)
	updateInterpolation(dt)

	// Clean up far chunks (Client-side Garbage Collection)
	// Render radius is roughly 16. Keep a bit more (e.g. 20) to avoid thrashing.
	// 5 seconds interval? Or every frame?
	// Every frame is fine, UnloadChunks is efficient enough (iterates map).
	// But let's do it every 60 frames to be safe on CPU.
	if rl.GetFrameTime() > 0 { // Just using valid time check, effectively always
		pPos := camera.Position
		cx := int(math.Floor(float64(pPos.X) / 16.0))
		cz := int(math.Floor(float64(pPos.Z) / 16.0))
		// Use a static counter to throttle
		// Accessing global or static var is ugly here, let's just run it. Map iteration of ~1000 items is fast.
		// Radius 24 chunks (16 render + 8 buffer)
		world.UnloadChunks(cx, cz, 24, func(chunkX, chunkZ int) {
			// Notify server that we unloaded this chunk
			// So it knows to resend if we return
			if client != nil {
				client.Send(&PacketUnloadChunk{CX: int32(chunkX), CZ: int32(chunkZ)})
			}
		})
	}
}

// Client-Pull: Request chunks we need but don't have
func requestMissingChunks() {
	if client == nil || world == nil {
		return
	}

	pPos := camera.Position
	cx := int(math.Floor(float64(pPos.X) / 16.0))
	cz := int(math.Floor(float64(pPos.Z) / 16.0))
	renderRadius := 16
	maxRequestsPerFrame := 32 // Increased for faster loading

	requestCount := 0
	radiusSq := renderRadius * renderRadius

	requestChunk := func(dx, dz int) bool {
		if dx*dx+dz*dz > radiusSq {
			return true // continue, not counted
		}
		chunkX, chunkZ := cx+dx, cz+dz
		key := chunkKey{X: chunkX, Z: chunkZ}
		chunk := world.getChunkIfGenerated(chunkX, chunkZ)
		if chunk != nil && chunk.generated {
			return true
		}
		if sent, pending := pendingChunkRequests[key]; pending && time.Since(sent) < 3*time.Second {
			return true
		}
		pendingChunkRequests[key] = time.Now()
		client.Send(&PacketChunkRequest{CX: int32(chunkX), CZ: int32(chunkZ)})
		requestCount++
		return requestCount < maxRequestsPerFrame
	}

	// Spiral out from center — only iterate ring edges
	if requestChunk(0, 0) {
		for r := 1; r <= renderRadius && requestCount < maxRequestsPerFrame; r++ {
			// Top and bottom edges
			for dx := -r; dx <= r && requestCount < maxRequestsPerFrame; dx++ {
				if !requestChunk(dx, -r) {
					break
				}
				if !requestChunk(dx, r) {
					break
				}
			}
			// Left and right edges (excluding corners already done)
			for dz := -r + 1; dz <= r-1 && requestCount < maxRequestsPerFrame; dz++ {
				if !requestChunk(-r, dz) {
					break
				}
				if !requestChunk(r, dz) {
					break
				}
			}
		}
	}
}

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
			rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
		}

	case *PacketChunkData:
		if world.applyChunkPacket(p) {
			delete(pendingChunkRequests, chunkKey{int(p.CX), int(p.CZ)})
		}

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

func updateEntities(dt float32) {
	// Not much to do here for now besides interpolation handled below
}

func updateInterpolation(dt float32) {
	for _, e := range remoteEntities {
		lerpFactor := float64(dt * 10.0)
		if lerpFactor > 1.0 {
			lerpFactor = 1.0
		}
		oldX, oldZ := e.X, e.Z
		e.X += (e.TX - e.X) * lerpFactor
		e.Y += (e.TY - e.Y) * lerpFactor
		e.Z += (e.TZ - e.Z) * lerpFactor
		if d, ok := mobContent.Definitions[e.MobKind]; ok {
			anim := mobContent.Animations[d.Animation]
			distance := float32(math.Hypot(e.X-oldX, e.Z-oldZ))
			if distance < 2 {
				e.AnimPhase += distance / anim.Stride * 2 * math.Pi
			}
			target := float32(0)
			if e.MobState == "walk" || e.MobState == "flee" {
				target = 1
			}
			e.AnimBlend += (target - e.AnimBlend) * min(dt*anim.BlendSpeed, float32(1))
			diff := float32(math.Atan2(math.Sin(float64(e.TargetYaw-e.Yaw)), math.Cos(float64(e.TargetYaw-e.Yaw))))
			e.Yaw += diff * float32(lerpFactor)
			e.MobHurt = max(0, e.MobHurt-dt)
			if e.MobHealth <= 0 {
				e.DeathTime += dt
			}
		}
	}
}

func drawGame() {
	camBlockX := int(math.Floor(float64(camera.Position.X) + 0.5))
	camBlockY := int(math.Floor(float64(camera.Position.Y) + 0.5))
	camBlockZ := int(math.Floor(float64(camera.Position.Z) + 0.5))
	inWater := world.BlockAt(camBlockX, camBlockY, camBlockZ) == blockWater

	rl.BeginMode3D(camera)

	// Draw World
	background := rl.NewColor(180, 210, 255, 255)
	if inWater {
		background = rl.NewColor(40, 70, 120, 255)
	}
	rl.ClearBackground(background)

	world.Draw(assets, camera)

	// Draw Entities
	for id, e := range remoteEntities {
		if id == *username {
			continue
		}
		if e.Type == EntityPlayer {
			drawCharacterModel(rl.NewVector3(float32(e.X), float32(e.Y), float32(e.Z)), e.Yaw)
		} else if e.Type == EntityItem {
			assets.DrawItem(e)
		} else if e.MobKind != "" && mobsRenderer != nil {
			mobsRenderer.Draw(e, input.ShowDebug)
		} else {
			pos := rl.NewVector3(float32(e.X), float32(e.Y)+0.5, float32(e.Z))
			rl.DrawCube(pos, 0.8, 0.8, 0.8, rl.Pink)
		}
	}

	rl.EndMode3D()

	// Draw Mining Crack Overlay
	world.DrawBlockCrack(assets, camera, input)

	// 2D Overlay
	if inWater {
		overlay := rl.NewColor(40, 90, 160, 120)
		rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), overlay)
	}

	if !input.InventoryOpen && !isPaused && !input.isDead() {
		assets.drawCrosshair()
	}

	if input.ShowDebug && perfMon != nil {
		m := perfMon.Metrics
		// Background for readability
		rl.DrawRectangle(5, 5, 550, 225, rl.Fade(invBackground, 0.92))

		rl.DrawText(fmt.Sprintf("%d FPS", rl.GetFPS()), 10, 10, 20, invAccent)
		rl.DrawText(fmt.Sprintf("Pos: %.1f, %.1f, %.1f", camera.Position.X, camera.Position.Y, camera.Position.Z), 10, 35, 20, invText)

		rl.DrawText(fmt.Sprintf("Chunks: %d loaded / %d mesh buffers", len(world.chunks), m.ActiveMeshes), 10, 60, 20, invText)
		rl.DrawText(fmt.Sprintf("%d chunk updates/sec", m.MeshesPerSec), 10, 85, 20, invText)
		rl.DrawText(fmt.Sprintf("Mem: %d MB (GC: %d)", m.HeapAllocMB, m.NumGC), 10, 110, 20, invText)
		rl.DrawText(fmt.Sprintf("Unloads/sec: %d", m.UnloadsPerSec), 10, 135, 20, invText)
		rl.DrawText(fmt.Sprintf("Frame p95/p99: %.1f/%.1f ms; Tick: %.1f ms", m.FrameP95, m.FrameP99, m.ServerTickMS), 10, 160, 18, invText)
		rl.DrawText(fmt.Sprintf("Mesh queue: %d/%d; Draws: %d", m.MeshJobs, m.MeshResults, m.DrawCalls), 10, 183, 18, invText)
		rl.DrawText(fmt.Sprintf("Triangles: %d", m.Triangles), 10, 206, 18, invText)
	}

	if input.InventoryOpen {
		assets.drawInventory(input)
	} else {
		assets.drawHotbar(input)
	}

	if !input.InventoryOpen {
		drawVitalsHUD(input)
	}
	if input.isDead() {
		drawDeathScreen(input)
		return
	}
	drawChatOverlay()

	if isPaused {
		drawPauseMenu()
	}
}

func drawCharacterModel(pos rl.Vector3, yaw float32) {
	rl.PushMatrix()
	rl.Translatef(pos.X, pos.Y, pos.Z)
	rl.Rotatef(-yaw, 0, 1, 0)

	// Body
	rl.DrawCube(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.DarkGray)
	rl.DrawCubeWires(rl.NewVector3(0, 0.7, 0), 0.6, 0.8, 0.3, rl.Black)

	// Head
	rl.DrawCube(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.LightGray)
	rl.DrawCubeWires(rl.NewVector3(0, 1.3, 0), 0.5, 0.5, 0.5, rl.Black)

	rl.PopMatrix()
}
