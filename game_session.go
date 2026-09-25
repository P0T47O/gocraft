package main

import (
	"fmt"
	"time"
)

func startGame(savePath string, ip string, isMultiplayer bool) {
	fmt.Println("Starting Game...")

	if assets == nil {
		assets = newWebGPUCPUAssets()
	}
	if err := ensureExperimentalWebGPURenderer(); err != nil {
		menuError = err.Error()
		return
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
		server.LocalCheats = true
		if err = server.ListenTCP("127.0.0.1:0"); err != nil {
			fmt.Printf("Cannot start local server: %v\n", err)
			server.World.Close()
			server = nil
			if !nativeWindowActive {
				closeExperimentalWebGPURenderer()
				assets.unload()
				assets = nil
			}
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
	resetChunkStreaming()
	clear(remoteEntities)
	explosionEffects = nil
	world = NewClientWorld()
	world.StartMeshWorkers(assets, 8)

	// Input
	input = NewInputState()
	input.InitFromCamera(camera)

	captureCursor()
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
		// A timeout must not let the process exit while disk writes are active.
		waitForServerSave(server.Done, 5*time.Second, func() {
			fmt.Println("Server: Still waiting for shutdown/save; do not force close.")
		})
		if server.ShutdownSaveError != nil {
			menuError = "World save failed. Check the console and save-error.log."
			fmt.Println(saveFailureMessage(server.ShutdownSaveError, "save-error.log"))
		}
		server = nil
	}
	if world != nil {
		// Chunk mesh handles must be released while their owning GPU device is
		// still alive. The experimental renderer is closed immediately afterward.
		world.Close()
		world = nil
	}
	if !nativeWindowActive {
		closeExperimentalWebGPURenderer()
	}
	if assets != nil && !nativeWindowActive {
		assets.unload()
		assets = nil
	}
	resetChunkStreaming()
	clear(remoteEntities)
	explosionEffects = nil
	// Reset State
	currentState = StateMenu
	menuPage = MenuMain
	isPaused = false
	pauseSettings = false
	releaseCursor()
}
