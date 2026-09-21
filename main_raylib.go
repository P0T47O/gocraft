package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"gocraft/platform"
)

// Legacy reference renderer, available only with -webgpu=false.
func runLegacyRaylib(settings *GameSettings) {
	rl.SetTraceLogLevel(rl.LogWarning) // Suppress INFO logs (texture, etc.)
	rl.InitWindow(int32(settings.ResolutionWidth), int32(settings.ResolutionHeight), "GoCraft")
	rl.SetExitKey(0) // Disable default ESC exit to allow custom Pause Menu
	defer rl.CloseWindow()

	var mobErr error
	if !*useWebGPU {
		mobsRenderer, mobErr = newMobRenderer(mobContent)
	}
	if mobErr != nil {
		fmt.Printf("Mob visuals unavailable; using placeholder: %v\n", mobErr)
	}
	defer func() { mobsRenderer.Close() }()
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
		captureRaylibInput()
		rl.BeginDrawing()
		rl.ClearBackground(invBackground)

		switch currentState {
		case StateMenu:
			updateMenu()
			drawMenu()
		case StatePlaying:
			setGameFrameTime(rl.GetFrameTime())
			updateGame()
			if currentState == StatePlaying {
				drawGame()
			}
		}

		rl.EndDrawing()
		perfMon.UpdateFrame(rl.GetFrameTime())
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
