package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	quitRequested bool
	pauseSettings bool
	worldScroll   int
	pendingDelete string
	menuError     string
)

func menuLayout() SurvivalLayout {
	return survivalLayout(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
}
func menuNavigate(page MenuPage) {
	menuPage = page
	ui.ActiveID = ""
	ui.DraggingID = ""
	menuError = ""
	pendingDelete = ""
}
func drawMenuBackdrop() {
	w, h := rl.GetScreenWidth(), rl.GetScreenHeight()
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), rl.NewColor(17, 29, 32, 255), invBackground)
	unit := max(8, float32(w)/100)
	// Quiet, deterministic voxel silhouettes: no external menu textures required.
	for layer := 0; layer < 3; layer++ {
		color := []rl.Color{rl.NewColor(32, 48, 47, 255), rl.NewColor(38, 57, 48, 255), rl.NewColor(28, 43, 38, 255)}[layer]
		step := unit * 4
		for x := float32(0); x < float32(w)+step; x += step {
			height := float32(h)*(0.69+float32(layer)*0.095) + float32(math.Sin(float64(x/float32(w))*9+float64(layer))*35+math.Sin(float64(x/float32(w))*21)*18)
			height = float32(int(height/unit)) * unit
			rl.DrawRectangleRec(rl.NewRectangle(x, height, step, float32(h)-height), color)
		}
	}
	l := menuLayout()
	for i := 0; i < 16; i++ {
		x := float32((i*173 + 27) % 980)
		y := float32((i*59 + 16) % 220)
		rl.DrawRectangleRec(l.Rect(x, y, 2, 2), rl.NewColor(91, 113, 95, 100))
	}
}
func menuPanel(l SurvivalLayout, title, subtitle string) {
	rl.DrawRectangleRec(l.Rect(6, 8, 1000, 620), rl.Fade(rl.Black, 0.25))
	inventoryBox(l.Rect(0, 0, 1000, 620), invBackground, invLine)
	rl.DrawRectangleRec(l.Rect(0, 0, 1000, 3), invAccent)
	l.Text("GOCRAFT", 28, 26, 16, invAccent)
	l.Text(title, 28, 67, 32, invText)
	l.Text(subtitle, 28, 110, 14, invMuted)
	rl.DrawRectangleRec(l.Rect(28, 144, 944, 1), invLine)
}
func menuMessage(l SurvivalLayout) {
	if menuError != "" {
		// Clip verbose network/filesystem errors to keep them within the panel.
		text := menuError
		for len(text) > 0 && float32(rl.MeasureText(text, int32(13*l.S))) > 930*l.S {
			text = string([]rune(text)[:len([]rune(text))-1])
		}
		l.Text(text, 28, 592, 13, invWarning)
	}
}
func drawMenu() {
	drawMenuBackdrop()
	l := menuLayout()
	switch menuPage {
	case MenuMain:
		l.Text("A WORLD OF YOUR OWN", 44, 126, 14, invAccent)
		l.Text("GOCRAFT", 40, 164, 66, invText)
		l.Text("Gather. Build. Make it yours.", 44, 250, 21, invMuted)
		for i, label := range []string{"EXPLORE", "CRAFT", "BUILD"} {
			x := float32(44 + i*150)
			inventoryBox(l.Rect(x, 304, 136, 32), rl.Fade(invPanel, 0.8), invLine)
			l.Text(label, x+16, 314, 12, invMuted)
		}
		// Small block monument ties the menu art to the game's grid.
		for i, height := range []float32{2, 4, 3, 5, 3, 2} {
			for y := float32(0); y < height; y++ {
				color := invPanel
				if y == height-1 {
					color = rl.NewColor(82, 104, 64, 255)
				}
				inventoryBox(l.Rect(46+float32(i)*54, 494-y*24, 52, 22), color, rl.NewColor(42, 56, 44, 255))
			}
		}
		inventoryBox(l.Rect(608, 110, 348, 410), invBackground, invLine)
		rl.DrawRectangleRec(l.Rect(608, 110, 348, 3), invAccent)
		l.Text("LET'S PLAY", 632, 140, 22, invText)
		l.Text("Your next adventure starts here.", 632, 174, 12, invMuted)
		if ui.DrawAction(l.Rect(632, 210, 300, 48), "Singleplayer", true, true) {
			menuNavigate(MenuSingleplayer)
			saveList = ScanSaves()
			worldScroll = 0
		}
		if ui.DrawButton(l.Rect(632, 272, 300, 48), "Multiplayer", true) {
			menuNavigate(MenuMultiplayer)
		}
		if ui.DrawButton(l.Rect(632, 334, 300, 48), "Settings", true) {
			menuNavigate(MenuSettings)
		}
		if ui.DrawButton(l.Rect(632, 428, 300, 42), "Quit game", true) {
			quitRequested = true
		}
		l.Text("EARLY ALPHA", 44, 580, 12, invMuted)
		l.Text("A voxel sandbox, built one block at a time.", 608, 580, 12, invMuted)
	case MenuSingleplayer:
		drawWorldMenu(l)
	case MenuCreateWorld:
		drawCreateMenu(l)
	case MenuMultiplayer:
		drawMultiplayerMenu(l)
	case MenuSettings:
		drawSettingsMenu(l, false)
	}
	menuMessage(l)
}
func drawWorldMenu(l SurvivalLayout) {
	menuPanel(l, "Your worlds", "Pick up where you left off, or start somewhere new.")
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), l.Rect(28, 160, 944, 350)) {
		worldScroll -= int(rl.GetMouseWheelMove())
	}
	worldScroll = max(0, min(worldScroll, max(0, len(saveList)-5)))
	if len(saveList) == 0 {
		l.Text("Your first world is waiting.", 58, 236, 24, invText)
		l.Text("Create a world to start gathering and building.", 58, 282, 17, invMuted)
	}
	for i := 0; i < 5 && worldScroll+i < len(saveList); i++ {
		save := saveList[worldScroll+i]
		y := float32(160 + i*68)
		rect := l.Rect(28, y, 944, 60)
		inventoryBox(rect, invPanel, invLine)
		rl.BeginScissorMode(int32(rect.X+12*l.S), int32(rect.Y), int32(598*l.S), int32(rect.Height))
		l.Text(save.Name, 44, y+10, 18, invText)
		detail := fmt.Sprintf("Seed %d", save.Seed)
		if save.IsLegacy {
			detail += "  /  Legacy world"
		}
		if !save.LastPlayed.IsZero() {
			detail += "  /  " + save.LastPlayed.Format("2006-01-02 15:04")
		}
		l.Text(detail, 44, y+37, 12, invMuted)
		rl.EndScissorMode()
		if ui.DrawAction(l.Rect(666, y+10, 138, 40), "Play", true, true) {
			startGame(save.Path, "", false)
			return
		}
		label := "Delete"
		confirm := pendingDelete != "" && pendingDelete == save.Path
		if confirm {
			label = "Confirm delete"
		}
		if ui.DrawButton(l.Rect(814, y+10, 146, 40), label, true) {
			if confirm {
				if err := DeleteSave(save.Path); err != nil {
					menuError = err.Error()
				} else {
					saveList = ScanSaves()
				}
				pendingDelete = ""
				return
			} else {
				pendingDelete = save.Path
			}
		}
	}
	l.Text(fmt.Sprintf("%d worlds  /  scroll to browse", len(saveList)), 28, 508, 12, invMuted)
	if pendingDelete != "" {
		l.Text("Click Confirm delete again to remove that world.", 370, 508, 12, invWarning)
	}
	if ui.DrawButton(l.Rect(28, 546, 220, 38), "Back", true) {
		menuNavigate(MenuMain)
	}
	if ui.DrawAction(l.Rect(706, 546, 266, 38), "Create new world", true, true) {
		menuNavigate(MenuCreateWorld)
	}
}
func drawCreateMenu(l SurvivalLayout) {
	menuPanel(l, "Create a world", "Start with empty hands and a landscape full of possibilities.")
	l.Text("MAKE IT YOURS", 28, 182, 14, invAccent)
	l.Text("Name your world, then", 28, 224, 17, invText)
	l.Text("choose a numeric seed.", 28, 252, 17, invText)
	l.Text("The same seed creates", 28, 306, 14, invMuted)
	l.Text("the same terrain.", 28, 330, 14, invMuted)
	l.Text("WORLD NAME", 354, 182, 12, invMuted)
	ui.DrawTextField(l.Rect(354, 208, 600, 46), &newWorldName, "world_name", 32, false)
	l.Text("WORLD SEED", 354, 288, 12, invMuted)
	ui.DrawTextField(l.Rect(354, 314, 600, 46), &newWorldSeed, "world_seed", 20, false)
	seed, err := strconv.ParseInt(strings.TrimSpace(newWorldSeed), 10, 64)
	if err != nil {
		l.Text("Enter a whole number for the seed.", 354, 378, 13, invWarning)
	}
	if ui.DrawButton(l.Rect(28, 546, 220, 38), "Back", true) {
		menuNavigate(MenuSingleplayer)
	}
	if ui.DrawAction(l.Rect(706, 546, 266, 38), "Create & play", strings.TrimSpace(newWorldName) != "" && err == nil, true) {
		path, e := CreateNewSave(strings.TrimSpace(newWorldName), seed)
		if e != nil {
			menuError = e.Error()
		} else {
			startGame(path, "", false)
		}
	}
}
func drawMultiplayerMenu(l SurvivalLayout) {
	menuPanel(l, "Play together", "Connect to a server and build a shared world.")
	l.Text("DIRECT CONNECTION", 28, 182, 14, invAccent)
	l.Text("Enter the address", 28, 226, 17, invText)
	l.Text("provided by your host.", 28, 254, 17, invText)
	l.Text("SERVER ADDRESS", 354, 182, 12, invMuted)
	ui.DrawTextField(l.Rect(354, 208, 600, 46), &ipInput, "server_ip", 128, false)
	l.Text("Example: 127.0.0.1:25565", 354, 274, 14, invMuted)
	l.Text("Playing as "+LoadSettings().PlayerName, 354, 334, 16, invAccent)
	if ui.DrawButton(l.Rect(28, 546, 220, 38), "Back", true) {
		menuNavigate(MenuMain)
	}
	if ui.DrawAction(l.Rect(706, 546, 266, 38), "Connect", strings.TrimSpace(ipInput) != "", true) {
		startGame("", strings.TrimSpace(ipInput), true)
	}
}

var menuResolutions = [][2]int{{1280, 720}, {1600, 900}, {1920, 1080}, {2560, 1440}, {1024, 768}, {1280, 800}, {2560, 1080}}

func drawSettingsMenu(l SurvivalLayout, fromPause bool) {
	menuPanel(l, "Settings", "Make yourself comfortable. Changes are saved automatically.")
	settings := LoadSettings()
	l.Text("PLAYER", 28, 184, 14, invAccent)
	l.Text("Your name is used to", 28, 222, 15, invMuted)
	l.Text("identify your inventory.", 28, 246, 15, invMuted)
	mipmapLabel := "Mipmaps: OFF"
	if settings.Mipmaps {
		mipmapLabel = "Mipmaps: ON"
	}
	if ui.DrawButton(l.Rect(28, 274, 142, 34), mipmapLabel, true) {
		settings.Mipmaps = !settings.Mipmaps
		SaveSettings()
		applyWorldTextureFiltering()
	}
	limit := supportedAnisotropy()
	af := min(normalizeAnisotropy(settings.Anisotropy), limit)
	afLabel := fmt.Sprintf("AF: %dx", af)
	if af == 1 {
		afLabel = "AF: OFF"
	}
	if limit == 1 {
		afLabel = "AF: N/A"
	}
	if ui.DrawButton(l.Rect(180, 274, 142, 34), afLabel, limit > 1) {
		settings.Anisotropy = af * 2
		if settings.Anisotropy > limit {
			settings.Anisotropy = 1
		}
		SaveSettings()
		applyWorldTextureFiltering()
	}
	l.Text("PLAYER NAME", 354, 164, 12, invMuted)
	ui.DrawTextField(l.Rect(354, 186, 600, 40), &settings.PlayerName, "player_name", 16, false)
	l.Text("MOUSE SENSITIVITY", 354, 244, 12, invMuted)
	ui.DrawSlider(l.Rect(354, 268, 600, 32), &settings.Sensitivity, 0.001, 0.02, "sensitivity")
	l.Text("DISPLAY RESOLUTION", 354, 322, 12, invMuted)
	for i, res := range menuResolutions {
		r := l.Rect(354+float32(i%3)*204, 344+float32(i/3)*48, 192, 38)
		selected := settings.ResolutionWidth == res[0] && settings.ResolutionHeight == res[1]
		if ui.DrawAction(r, fmt.Sprintf("%d x %d", res[0], res[1]), true, selected) {
			settings.ResolutionWidth = res[0]
			settings.ResolutionHeight = res[1]
			SaveSettings()
			ApplySettings()
			return
		}
	}
	l.Text("CONTROLS", 28, 328, 14, invAccent)
	l.Text(fmt.Sprintf("RENDER DISTANCE: %d CHUNKS (%d BLOCKS)", settings.RenderDistance, settings.RenderDistance*chunkWidth), 354, 494, 12, invMuted)
	distance := float32(settings.RenderDistance)
	ui.DrawSlider(l.Rect(354, 514, 600, 24), &distance, minRenderDistance, maxRenderDistance, "render_distance")
	settings.RenderDistance = clampRenderDistance(int(distance + .5))
	for i, line := range []string{"WASD  Move", "Space  Jump / swim up", "Ctrl + W  Sprint", "Shift  Sneak / dive", "E  Inventory", "Esc  Pause / back"} {
		l.Text(line, 28, 364+float32(i)*26, 14, invMuted)
	}
	if rl.IsMouseButtonReleased(rl.MouseLeftButton) || rl.IsKeyPressed(rl.KeyEnter) {
		SaveSettings()
		if input != nil {
			input.Sensitivity = settings.Sensitivity
		}
	}
	if ui.DrawAction(l.Rect(706, 546, 266, 38), "Done", true, true) {
		SaveSettings()
		if currentState == StateMenu {
			*username = settings.PlayerName
		}
		if fromPause {
			pauseSettings = false
			ui.ActiveID = ""
		} else {
			menuNavigate(MenuMain)
		}
	}
}
func drawPauseMenu() {
	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(9, 17, 22, 195))
	l := menuLayout()
	if pauseSettings {
		drawSettingsMenu(l, true)
		return
	}
	inventoryBox(l.Rect(260, 108, 480, 408), invBackground, invLine)
	rl.DrawRectangleRec(l.Rect(260, 108, 480, 3), invAccent)
	l.Text("GOCRAFT", 292, 140, 13, invAccent)
	l.Text("Take a moment.", 292, 174, 30, invText)
	label := "Your world is right here."
	if server == nil {
		label = "Connected server remains active."
	}
	l.Text(label, 292, 222, 14, invMuted)
	if ui.DrawAction(l.Rect(292, 264, 416, 46), "Resume game", true, true) {
		isPaused = false
		if server != nil {
			server.Paused.Store(false)
		}
		input.SkipCamera = true
		rl.DisableCursor()
	}
	if ui.DrawButton(l.Rect(292, 324, 416, 46), "Settings", true) {
		pauseSettings = true
		ui.ActiveID = ""
	}
	quitLabel := "Save & return to menu"
	if server == nil {
		quitLabel = "Disconnect & return to menu"
	}
	if ui.DrawButton(l.Rect(292, 398, 416, 46), quitLabel, true) {
		exitGame()
		pauseSettings = false
		return
	}
	l.Text("ESC  Resume", 292, 474, 12, invMuted)
}
