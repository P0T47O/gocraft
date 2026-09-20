package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"time"
)

func startGame(savePath string, ip string, isMultiplayer bool) {
	fmt.Println("Starting Game...")

	// Load Common Assets
	// chunkPool = NewChunkPool(1024) // Share the global pool? Or new? Global is fine.
	// initBlockRegistry() // Already initialized in main()
	if *useWebGPU {
		assets = newWebGPUCPUAssets()
		if err := ensureExperimentalWebGPURenderer(); err != nil {
			fmt.Printf("WebGPU initialization failed, using OpenGL: %v\n", err)
			closeExperimentalWebGPURenderer()
			*useWebGPU = false
			assets = loadRenderAssets()
		}
	} else {
		assets = loadRenderAssets()
	}
	if !*useWebGPU && mobsRenderer == nil {
		var err error
		mobsRenderer, err = newMobRenderer(mobContent)
		if err != nil {
			fmt.Printf("Mob visuals unavailable: %v\n", err)
		}
	}

	// Initialize Camera
	camera = gameCamera{
		Position: newGameVec3(8, 8, 20),
		Target:   newGameVec3(8, 2, 8),
		Up:       newGameVec3(0, 1, 0),
		Fovy:     70,
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
			closeExperimentalWebGPURenderer()
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
		// Chunk mesh handles must be released while their owning GPU device is
		// still alive. The experimental renderer is closed immediately afterward.
		world.Close()
		world = nil
	}
	closeExperimentalWebGPURenderer()
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
