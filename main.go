package main

import (
	"flag"
	"fmt"
	"gocraft/platform"
	"math"
	"os"
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
	pendingChunkRequests = make(map[chunkKey]bool) // Track in-flight chunk requests

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
	currentGameMode byte = ModeCreative

	// Chat
	isChatOpen  bool   = false
	chatInput   string = ""
	chatHistory []string
)

type RemoteEntity struct {
	ID         string
	Type       EntityType
	X, Y, Z    float64 // Visual position (Lerped)
	TX, TY, TZ float64 // Target position
	Yaw, Pitch float32
	Metadata   int32
}

func main() {
	flag.Parse()

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

	platform.InitGLOnce()
	rl.SetTargetFPS(60)

	// Initialize block definitions (CRITICAL: must be called before any world/rendering)
	initBlockRegistry()

	// Resources
	ui = NewUIComponents()
	perfMon = NewPerformanceMonitor()
	defer perfMon.Close()

	if *username == "" || (*username)[:6] == "Player" { // Heuristic: If default or generic, use settings
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

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		switch currentState {
		case StateMenu:
			updateMenu()
			drawMenu()
		case StatePlaying:
			updateGame()
			drawGame()
		}

		rl.EndDrawing()
	}

	// Cleanup on Exit
	if currentState == StatePlaying {
		exitGame()
	}
}

func updateMenu() {
	rl.ShowCursor()

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

var logoTexture rl.Texture2D

func drawMenu() {
	if logoTexture.ID == 0 {
		// Load the transparent logo directly
		logoTexture = rl.LoadTexture("assets/logo.png")
	}

	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())

	// Background
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), rl.SkyBlue, rl.DarkBlue)

	btnWidth := float32(300)
	btnHeight := float32(50)
	centerX := (w - btnWidth) / 2
	startY := h * 0.35

	switch menuPage {
	case MenuMain:
		if logoTexture.ID != 0 {
			// Calculate Logo Scale to fit nicely
			// Max width: 60% of screen
			// Max height: 20% of screen (to fit before startY=35%)
			maxLogoW := w * 0.6
			maxLogoH := h * 0.2

			scaleW := maxLogoW / float32(logoTexture.Width)
			scaleH := maxLogoH / float32(logoTexture.Height)

			// Use the smaller scale to maintain aspect ratio
			scalar := scaleW
			if scaleH < scaleW {
				scalar = scaleH
			}
			// Don't upscale pixel art too much if not needed, but here we want it big enough
			if scalar < 0.5 {
				scalar = 0.5
			}

			logoW := float32(logoTexture.Width) * scalar
			logoH := float32(logoTexture.Height) * scalar

			logoX := (w - logoW) / 2
			// Position: Centered in the top area available (0 to startY)
			// Available height = startY = h * 0.35
			// Center of top area = startY / 2
			logoY := (startY - logoH) / 2

			rl.DrawTextureEx(logoTexture, rl.Vector2{X: logoX, Y: logoY}, 0, scalar, rl.White)
		} else {
			ui.DrawLabel(w/2-100, h*0.15, "GoCraft", 60, rl.White)
		}

		if ui.DrawButton(rl.NewRectangle(centerX, startY, btnWidth, btnHeight), "Singleplayer", true) {
			menuPage = MenuSingleplayer
			saveList = ScanSaves()
		}
		if ui.DrawButton(rl.NewRectangle(centerX, startY+70, btnWidth, btnHeight), "Multiplayer", true) {
			menuPage = MenuMultiplayer
		}
		if ui.DrawButton(rl.NewRectangle(centerX, startY+140, btnWidth, btnHeight), "Settings", true) {
			menuPage = MenuSettings
		}
		if ui.DrawButton(rl.NewRectangle(centerX, startY+210, btnWidth, btnHeight), "Quit", true) {
			os.Exit(0)
		}

	case MenuSingleplayer:
		ui.DrawLabel(centerX, h*0.25, "Select World", 30, rl.White)

		listY := h * 0.35
		for _, save := range saveList {
			label := fmt.Sprintf("%s (Seed: %d)", save.Name, save.Seed)
			if save.IsLegacy {
				label += " [Legacy]"
			}
			if ui.DrawButton(rl.NewRectangle(centerX, listY, btnWidth, btnHeight), label, true) {
				startGame(save.Path, "", false)
			}
			// Delete Button small
			if ui.DrawButton(rl.NewRectangle(centerX+btnWidth+10, listY, 80, btnHeight), "Delete", true) {
				DeleteSave(save.Path)
				saveList = ScanSaves() // Refresh
			}
			listY += 60
		}

		if ui.DrawButton(rl.NewRectangle(centerX, h*0.75, btnWidth, btnHeight), "Create New World", true) {
			menuPage = MenuCreateWorld
		}
		if ui.DrawButton(rl.NewRectangle(centerX, h*0.85, btnWidth, btnHeight), "Back", true) {
			menuPage = MenuMain
		}

	case MenuCreateWorld:
		ui.DrawLabel(centerX, h*0.3, "World Name", 20, rl.White)
		ui.DrawTextField(rl.NewRectangle(centerX, h*0.35, btnWidth, 40), &newWorldName, "world_name", 20, false)

		ui.DrawLabel(centerX, h*0.45, "Seed (Number)", 20, rl.White)
		ui.DrawTextField(rl.NewRectangle(centerX, h*0.5, btnWidth, 40), &newWorldSeed, "world_seed", 10, false)

		if ui.DrawButton(rl.NewRectangle(centerX, h*0.7, btnWidth, btnHeight), "Create & Play", len(newWorldName) > 0) {
			var seed int64 = 12345
			fmt.Sscanf(newWorldSeed, "%d", &seed)
			path, err := CreateNewSave(newWorldName, seed)
			if err == nil {
				startGame(path, "", false)
			} else {
				fmt.Printf("Error creating save: %v\n", err)
			}
		}
		if ui.DrawButton(rl.NewRectangle(centerX, h*0.85, btnWidth, btnHeight), "Cancel", true) {
			menuPage = MenuSingleplayer
		}

	case MenuMultiplayer:
		ui.DrawLabel(centerX, h*0.3, "Server IP", 20, rl.White)
		ui.DrawTextField(rl.NewRectangle(centerX, h*0.35, btnWidth, 40), &ipInput, "server_ip", 30, false)

		if ui.DrawButton(rl.NewRectangle(centerX, h*0.5, btnWidth, btnHeight), "Connect", true) {
			startGame("", ipInput, true)
		}
		if ui.DrawButton(rl.NewRectangle(centerX, h*0.85, btnWidth, btnHeight), "Back", true) {
			menuPage = MenuMain
		}

	case MenuSettings:
		settings := LoadSettings() // Returns singleton cached if already loaded
		ui.DrawLabel(centerX, h*0.2, "Settings", 40, rl.White)

		// Resolution
		// Resolution Options
		resOptions := []string{
			"1280x720 (16:9)",
			"1600x900 (16:9)",
			"1920x1080 (16:9)",
			"2560x1440 (16:9)",
			"1024x768 (4:3)",
			"1280x800 (16:10)",
			"2560x1080 (21:9)",
		}

		// Find current index
		currentRes := fmt.Sprintf("%dx%d", settings.ResolutionWidth, settings.ResolutionHeight)
		selectedIndex := 0
		for i, opt := range resOptions {
			if len(opt) >= len(currentRes) && opt[:len(currentRes)] == currentRes {
				selectedIndex = i
				break
			}
		}

		// Z-Order: Draw dropdown LAST in the frame (after others) so it appears on top.
		// However, immediate mode is top-down.
		// For simple fix with no layering system: Draw everything else first, then dropdown?
		// But layout is sequential.
		// Actually, DrawDropdown draws the header first. The expansion is drawn below.
		// If sensitivity slider is below, dropdown expansion might be covered by it or cover it depending on draw order.
		// Raylib draws painters algorithm (last on top).
		// So if we draw dropdown here, and then sensitivity slider below it...
		// The dropdown OPTIONS (drawn inside DrawDropdown) will be drawn NOW.
		// Use a deferred approach or specific Z-handling?
		// For now, let's just draw it. If it overlaps sensitivity, sensitivity might draw ON TOP of dropdown options.
		// To fix: Draw Sensitivity FIRST, then Dropdown?
		// Layout Y positions matter.
		// Let's modify the order: Draw Sensitivity, THEN Resolution Dropdown (which is visually above, but drawn later).
		// Sensitivity Y=330. Resolution Y=220.
		// If we draw Sensitivity (330) first, then Resolution (220) + Options (dropping down to >260)...
		// Resolution + Options will be ON TOP of Sensitivity.
		// Perfect.

		// Player Name
		dropdownOpen := ui.ActiveID == "res_dropdown"
		ui.DrawLabel(centerX, h*0.35, "Player Name", 20, rl.White)
		ui.DrawTextField(rl.NewRectangle(centerX, h*0.4, btnWidth, 30), &settings.PlayerName, "player_name", 16, dropdownOpen)

		// Sensitivity (Draw First to be partially occluded by dropdown if needed)
		ui.DrawLabel(centerX, h*0.45, fmt.Sprintf("Sensitivity: %.4f", settings.Sensitivity), 20, rl.White)
		ui.DrawSlider(rl.NewRectangle(centerX, h*0.5, btnWidth, 20), &settings.Sensitivity, 0.001, 0.02, "sens_slider")

		// Auto-save settings on change
		if rl.IsMouseButtonReleased(rl.MouseLeftButton) || rl.IsKeyPressed(rl.KeyEnter) {
			SaveSettings()
		}

		// Back Button (Draw First to be behind dropdown)
		if ui.DrawButton(rl.NewRectangle(centerX, h*0.85, btnWidth, btnHeight), "Back", true) {
			menuPage = MenuMain
		}

		// Resolution Dropdown (Draw Last)
		if ui.DrawDropdown(rl.NewRectangle(centerX, h*0.3, btnWidth, btnHeight), resOptions, &selectedIndex, "res_dropdown") {
			// Apply Selection
			switch selectedIndex {
			case 0:
				settings.ResolutionWidth, settings.ResolutionHeight = 1280, 720
			case 1:
				settings.ResolutionWidth, settings.ResolutionHeight = 1600, 900
			case 2:
				settings.ResolutionWidth, settings.ResolutionHeight = 1920, 1080
			case 3:
				settings.ResolutionWidth, settings.ResolutionHeight = 2560, 1440
			case 4:
				settings.ResolutionWidth, settings.ResolutionHeight = 1024, 768
			case 5:
				settings.ResolutionWidth, settings.ResolutionHeight = 1280, 800
			case 6:
				settings.ResolutionWidth, settings.ResolutionHeight = 2560, 1080
			}
			SaveSettings()
			ApplySettings()
		}
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
		go server.StartTCP("127.0.0.1:25566")

		// Wait for socket to listen
		time.Sleep(200 * time.Millisecond)
		ip = "127.0.0.1:25566"
	}

	fmt.Printf("Connecting to %s...\n", ip)
	client, err = ConnectTCP(ip, *username)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		assets.unload()
		return
	}

	// Client World
	world = NewClientWorld()
	world.StartMeshWorkers(assets, 8)

	// Input
	input = NewInputState()
	input.InitFromCamera(camera)

	rl.DisableCursor()
	isPaused = false
	currentState = StatePlaying
}

func exitGame() {
	if client != nil {
		client.Conn.Close()
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
	if assets != nil {
		assets.unload()
	}
	// Reset State
	currentState = StateMenu
	menuPage = MenuMain
	isPaused = false
	rl.EnableCursor()
}

func updateGame() {
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
	} else if rl.IsKeyPressed(rl.KeyEnter) {
		isChatOpen = true
		rl.EnableCursor()
		// rl.SetMousePosition? No, let cursor be free.
		return
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		if input.InventoryOpen {
			input.InventoryOpen = false
			if !isPaused {
				rl.DisableCursor()
			}
		} else {
			isPaused = !isPaused
			if isPaused {
				rl.ShowCursor()
			} else {
				rl.DisableCursor()
				// Reset mouse to center to prevent view jump
				rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
			}
		}
	}

	// GameMode Switch (F1)
	if rl.IsKeyPressed(rl.KeyF1) {
		newMode := byte(ModeCreative)
		if currentGameMode == ModeCreative {
			newMode = ModeSurvival
		}
		// Request mode switch
		client.Send(&PacketGameMode{Mode: newMode})
	}

	// Inventory toggle is handled in HandleInput -> ToggleInventory

	// Singleplayer Pause: Freeze update loop
	if isPaused && server != nil {
		return
	}

	dt := rl.GetFrameTime()
	perfMon.Update()
	assets.Update(dt)

	// Packet Loop
Loop:
	for {
		select {
		case pkt := <-client.Incoming:
			handlePacket(pkt)
		default:
			break Loop
		}
	}

	world.ProcessMeshResults(assets, 512)

	HandleInput(world, &camera, input, client)
	world.ProcessImmediateMeshes(assets, 16)

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

	// Check chunks in render distance (spiral out from center)
	for r := 0; r <= renderRadius && requestCount < maxRequestsPerFrame; r++ {
		for dz := -r; dz <= r && requestCount < maxRequestsPerFrame; dz++ {
			for dx := -r; dx <= r && requestCount < maxRequestsPerFrame; dx++ {
				// Only check edge of current ring (optimization)
				if r > 0 && dx > -r && dx < r && dz > -r && dz < r {
					continue
				}

				// Match render distance: use circular (not square) range
				if dx*dx+dz*dz > renderRadius*renderRadius {
					continue
				}

				chunkX, chunkZ := cx+dx, cz+dz
				key := chunkKey{X: chunkX, Z: chunkZ}

				// Skip if already have this chunk
				chunk := world.getChunkIfGenerated(chunkX, chunkZ)
				if chunk != nil && chunk.generated {
					continue
				}

				// Skip if already requested (but don't skip forever - re-request if too long)
				if pendingChunkRequests[key] {
					continue
				}

				// Request the chunk from server
				pendingChunkRequests[key] = true
				client.Send(&PacketChunkRequest{CX: int32(chunkX), CZ: int32(chunkZ)})
				requestCount++
			}
		}
	}
}

func handlePacket(pkt Packet) {
	switch p := pkt.(type) {
	case *PacketChunkData:
		cx, cz := int(p.CX), int(p.CZ)
		// Clear pending request
		delete(pendingChunkRequests, chunkKey{X: cx, Z: cz})
		chunk := world.requestChunk(cx, cz)
		chunk.mu.Lock()
		idx := 0
		for lx := 0; lx < chunkWidth; lx++ {
			for y := 0; y < chunkHeight; y++ {
				for lz := 0; lz < chunkWidth; lz++ {
					chunk.blocks[lx][y][lz] = p.Data[idx]
					l := p.LightData[idx]
					chunk.skyLight[lx][y][lz] = l >> 4
					chunk.blockLight[lx][y][lz] = l & 0x0F
					idx++
				}
			}
		}
		chunk.rebuildHeightMap()
		chunk.rebuildTorchCount()
		chunk.generated = true
		chunk.dirty = true
		ensureChunkSections(chunk)
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
			chunk.meshVersion[i]++
		}
		chunk.mu.Unlock()
		world.markNeighborsDirty(cx, cz)

	case *PacketBlockChange:
		world.SetBlockAt(int(p.X), int(p.Y), int(p.Z), p.BlockID)

	case *PacketLogin:
		world.seed = p.Seed
		fmt.Printf("Synced with server seed: %d\n", p.Seed)

	case *PacketSpawnPoint:
		camera.Position = rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
		camera.Target = rl.NewVector3(camera.Position.X, camera.Position.Y-2, camera.Position.Z+5)
		input.InitFromCamera(camera)

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

	case *PacketInventoryUpdate:
		if p.SlotID == -1 {
			// Update Cursor Item (Held on mouse)
			input.CursorItem = Item{ID: p.ItemID, Count: p.Count}
		} else if p.SlotID >= 0 && p.SlotID < 36 {
			// Update the authoritative inventory
			if client != nil {
				client.Inventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count}
			} else {
				localInventory.Slots[p.SlotID] = Item{ID: p.ItemID, Count: p.Count}
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
		e.X += (e.TX - e.X) * lerpFactor
		e.Y += (e.TY - e.Y) * lerpFactor
		e.Z += (e.TZ - e.Z) * lerpFactor
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
		} else {
			pos := rl.NewVector3(float32(e.X), float32(e.Y)+0.5, float32(e.Z))
			rl.DrawCube(pos, 0.8, 0.8, 0.8, rl.Pink)
		}
	}

	rl.EndMode3D()

	// 2D Overlay
	if inWater {
		overlay := rl.NewColor(40, 90, 160, 120)
		rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), overlay)
	}

	assets.drawCrosshair()

	if input.ShowDebug {
		m := perfMon.Metrics
		// Background for readability
		rl.DrawRectangle(5, 5, 400, 150, rl.NewColor(0, 0, 0, 100))

		rl.DrawFPS(10, 10)
		rl.DrawText(fmt.Sprintf("Pos: %.1f, %.1f, %.1f", camera.Position.X, camera.Position.Y, camera.Position.Z), 10, 35, 20, rl.White)

		rl.DrawText(fmt.Sprintf("Chunks: %d loaded / %d mesh buffers", len(world.chunks), m.ActiveMeshes), 10, 60, 20, rl.White)
		rl.DrawText(fmt.Sprintf("%d chunk updates/sec", m.MeshesPerSec), 10, 85, 20, rl.White)
		rl.DrawText(fmt.Sprintf("Mem: %d MB (GC: %d)", m.HeapAllocMB, m.NumGC), 10, 110, 20, rl.White)
		rl.DrawText(fmt.Sprintf("Unloads/sec: %d", m.UnloadsPerSec), 10, 135, 20, rl.White)
	}

	if input.InventoryOpen {
		assets.drawInventory(input)
	} else {
		assets.drawHotbar(input)
	}

	// Chat UI
	if len(chatHistory) > 0 || isChatOpen {
		// Draw History
		historyH := int32(len(chatHistory) * 20)
		baseY := int32(rl.GetScreenHeight()) - 50 - historyH
		if baseY < int32(rl.GetScreenHeight())/2 {
			baseY = int32(rl.GetScreenHeight()) / 2
		}

		rl.DrawRectangle(5, baseY-5, 500, historyH+10, rl.NewColor(0, 0, 0, 100))
		for i, msg := range chatHistory {
			rl.DrawText(msg, 10, baseY+int32(i*20), 18, rl.White)
		}
	}

	if isChatOpen {
		// Draw Input
		baseY := int32(rl.GetScreenHeight()) - 40
		rl.DrawRectangle(5, baseY, 500, 30, rl.NewColor(0, 0, 0, 200))
		rl.DrawText("> "+chatInput+"_", 10, baseY+6, 20, rl.Yellow)
	}

	if isPaused {
		rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(0, 0, 0, 150))

		w := float32(rl.GetScreenWidth())
		h := float32(rl.GetScreenHeight())
		centerX := w / 2
		centerY := h / 2
		btnWidth := float32(300)
		btnHeight := float32(50)

		text := "Game Paused"
		textW := rl.MeasureText(text, 40)
		ui.DrawLabel(centerX-float32(textW)/2, centerY-100, text, 40, rl.White)

		if ui.DrawButton(rl.NewRectangle(centerX-btnWidth/2, centerY, btnWidth, btnHeight), "Save & Quit", true) {
			exitGame()
		}
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
