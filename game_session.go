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
	assets = loadRenderAssets()

	// Gameplay owns the camera state. Raylib keeps only the legacy/reference
	// presentation mirror until the window/input boundary is replaced.
	initialPosition := gameVec3{X: 8, Y: 8, Z: 20}
	initialTarget := gameVec3{X: 8, Y: 2, Z: 8}
	camera = rl.Camera3D{
		Position:   raylibVec3FromGame(initialPosition),
		Target:     raylibVec3FromGame(initialTarget),
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

	input = NewInputState()
	input.InitFromCamera(initialPosition, initialTarget)

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
