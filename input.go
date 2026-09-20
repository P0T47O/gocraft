package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	playerRadius = 0.3
	playerHeight = 1.8
	playerEyeY   = 1.62

	// Movement speeds (blocks/sec)
	walkSpeed = 4.3
	runSpeed  = 6.45 // 1.5x walk speed
	flySpeed  = 30.0

	// Physics constants
	gravity      = 32.0 // blocks/sec²
	jumpVelocity = 8.5  // Initial jump velocity (~1.25 blocks high)
	terminalVel  = 78.0 // Terminal falling velocity

	// Double-tap detection
	doubleTapTime = 0.3 // seconds
)

type InputState struct {
	ClosedContainerToken int32
	Container            *PacketContainerState
	CurrentIndex         int
	CurrentBlock         byte
	InventoryOpen        bool
	SelectedSlot         int
	Hotbar               [9]byte
	SkipCamera           bool
	Yaw                  float32
	Pitch                float32
	Sensitivity          float32
	MoveSpeed            float32
	CursorItem           Item // Held item on mouse cursor
	InventoryPage        int
	ShowDebug            bool
	CraftingStation      byte // 0: None, 1: Workbench

	// Survival Mode Physics
	VelocityY       float32 // Vertical velocity
	OnGround        bool    // Is player standing on solid ground
	LastWTime       float64 // Time of last W key press (for double-tap detection)
	IsRunning       bool    // Current movement state
	SprintLatched   bool
	IsSneaking      bool
	IsSwimming      bool
	Vitals          PacketVitals
	VitalsReady     bool
	HurtFlash       float32
	RespawnWaiting  bool
	RespawnLast     float64
	AwaitingTerrain bool

	// Mining System
	MiningTarget   *hitInfo // Block currently being mined (can be nil)
	MiningTool     byte
	MiningBlock    byte
	MiningProgress float32 // 0.0 to 1.0
	LastMiningTime float64 // Time of last frame's mining logic
	LastBreakTime  float64 // Time of last block break (Creative delay)

	// Crafting UI State
	CraftingScroll   int   // Number of recipe rows scrolled down
	RecipeSelected   int32 // Result ID, stable across ingredient variants
	RecipesReadyOnly bool
}

func NewInputState() *InputState {
	state := &InputState{CurrentIndex: 0, SelectedSlot: 0}
	for i := 0; i < len(state.Hotbar); i++ {
		if i < len(allBlocks) {
			state.Hotbar[i] = allBlocks[i]
		} else {
			state.Hotbar[i] = blockAir
		}
	}
	state.CurrentBlock = state.Hotbar[state.SelectedSlot]

	settings := LoadSettings()
	state.Sensitivity = settings.Sensitivity

	state.MoveSpeed = 30.0
	return state
}

func (s *InputState) ToggleInventory() {
	if s.Container != nil && (rl.IsKeyPressed(rl.KeyE) || rl.IsKeyPressed(rl.KeyEscape)) {
		s.closeContainerUI()
	}
	if rl.IsKeyPressed(rl.KeyE) {
		s.InventoryOpen = !s.InventoryOpen
		s.SkipCamera = true
		if s.InventoryOpen {
			rl.EnableCursor()
			rl.SetMousePosition(rl.GetScreenWidth()/2, rl.GetScreenHeight()/2)
		} else {
			s.CraftingStation = 0 // Clear crafting state
			rl.SetMousePosition(rl.GetScreenWidth()/2, rl.GetScreenHeight()/2)
			rl.DisableCursor()
		}
	}
	if s.InventoryOpen && rl.IsKeyPressed(rl.KeyEscape) {
		s.InventoryOpen = false
		s.SkipCamera = true
		s.CraftingStation = 0

		rl.SetMousePosition(rl.GetScreenWidth()/2, rl.GetScreenHeight()/2)
		rl.DisableCursor()
	}
}

func (s *InputState) UpdateCamera(world *World, camera *gameCamera) {
	delta := rl.GetMouseDelta()
	s.Yaw -= delta.X * s.Sensitivity
	s.Pitch -= delta.Y * s.Sensitivity
	if s.Pitch > 1.55 {
		s.Pitch = 1.55
	} else if s.Pitch < -1.55 {
		s.Pitch = -1.55
	}

	forward := newGameVec3(
		float32(math.Sin(float64(s.Yaw)))*float32(math.Cos(float64(s.Pitch))),
		float32(math.Sin(float64(s.Pitch))),
		float32(math.Cos(float64(s.Yaw)))*float32(math.Cos(float64(s.Pitch))),
	)
	forward = gameVec3Normalize(forward)
	up := newGameVec3(0, 1, 0)

	controls := MovementControls{}
	if rl.IsKeyPressed(rl.KeyW) {
		now := rl.GetTime()
		if s.LastWTime > 0 && now-s.LastWTime < doubleTapTime {
			s.SprintLatched = true
		}
		s.LastWTime = now
	}
	if !rl.IsKeyDown(rl.KeyW) {
		s.SprintLatched = false
	}
	if rl.IsKeyDown(rl.KeyW) {
		controls.Forward++
	}
	if rl.IsKeyDown(rl.KeyS) {
		controls.Forward--
	}
	if rl.IsKeyDown(rl.KeyD) {
		controls.Side++
	}
	if rl.IsKeyDown(rl.KeyA) {
		controls.Side--
	}
	controls.Jump = rl.IsKeyDown(rl.KeySpace)
	controls.Sneak = rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
	controls.Sprint = s.SprintLatched || rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
	camera.Position = s.stepMovementCore(world, camera.Position, gameFrameTime(), controls, currentGameMode == ModeCreative)

	camera.Target = gameVec3Add(camera.Position, forward)
	camera.Up = up
}

func (s *InputState) UpdateSelection(allowWheel bool) {
	if allowWheel {
		wheel := rl.GetMouseWheelMove()
		if wheel > 0 {
			s.SelectedSlot--
		} else if wheel < 0 {
			s.SelectedSlot++
		}
		if s.SelectedSlot < 0 {
			s.SelectedSlot = 0
		}
		if s.SelectedSlot >= len(s.Hotbar) {
			s.SelectedSlot = len(s.Hotbar) - 1
		}
		if client != nil {
			client.Send(&PacketSlotChange{Slot: int32(s.SelectedSlot)})
		}
	}
	for i := 0; i < 9; i++ {
		if rl.IsKeyPressed(int32(rl.KeyOne + int32(i))) {
			if i < len(s.Hotbar) {
				s.SelectedSlot = i
				if client != nil {
					client.Send(&PacketSlotChange{Slot: int32(i)})
				}
			}
			break
		}
	}
	s.CurrentBlock = s.Hotbar[s.SelectedSlot]
}

func resolveCollision(world *World, pos, delta rl.Vector3) rl.Vector3 {
	feet := pos
	feet.Y -= playerEyeY
	result := moveCollider(world, feet, delta, Collider{Width: playerRadius * 2, Depth: playerRadius * 2, Height: playerHeight})
	result.Y += playerEyeY
	return result
}
func collides(world *World, pos rl.Vector3) bool {
	pos.Y -= playerEyeY
	return colliderHits(world, pos, Collider{Width: playerRadius * 2, Depth: playerRadius * 2, Height: playerHeight})
}

func isSolidBlock(b byte) bool {
	if b == blockAir {
		return false
	}
	def := GetBlock(b)
	return def.IsCollidable
}

func blockIndexFromCoord(v float32) int {
	return int(math.Floor(float64(v) + 0.5))
}

func HandleInput(world *World, camera *gameCamera, state *InputState, client *Client) hitInfo {
	if rl.IsKeyPressed(rl.KeyF3) {
		state.ShowDebug = !state.ShowDebug
	}
	if rl.IsKeyPressed(rl.KeyF1) {
		if currentGameMode == ModeCreative {
			currentGameMode = ModeSurvival
		} else {
			currentGameMode = ModeCreative
		}
		if client != nil {
			client.Send(&PacketGameMode{Mode: byte(currentGameMode)})
		}
	}
	state.ToggleInventory()
	if state.SkipCamera {
		rl.GetMouseDelta()
		state.SkipCamera = false
	}
	if state.InventoryOpen {
		old := camera.Position
		camera.Position = state.stepMovementCore(world, old, gameFrameTime(), MovementControls{}, currentGameMode == ModeCreative)
		camera.Target = gameVec3Add(camera.Target, gameVec3Subtract(camera.Position, old))
		state.UpdateSelection(false)
		state.UpdateInventoryPage()
		state.UpdateInventorySelection(client)
		return hitInfo{}
	}

	state.UpdateCamera(world, camera)
	state.UpdateSelection(true)

	if rl.IsKeyPressed(rl.KeyQ) {
		item := state.Hotbar[state.SelectedSlot]
		if item != blockAir && client != nil {
			// Send Drop Packet
			// Value = (Count=1 << 8) | ItemID
			val := int32(1<<8) | int32(item)
			client.Send(&PacketPlayerAction{
				ActionType: 0,
				Value:      val,
			})
		}
	}

	rayOrigin, rayDirection := state.RayFromCenter(*camera)

	// Reach distance depends on Gamemode
	reachDist := float32(8.0) // Creative default
	if currentGameMode == ModeSurvival {
		reachDist = 3.0
	}

	hit := world.HitTest(rayOrigin, rayDirection, reachDist)
	mobTarget := ""
	nearest := float32(3.5)
	if hit.hit {
		nearest = min(nearest, hit.distance)
	}

	// Keep client target selection renderer-neutral too. This mirrors the
	// authoritative server slab test, so Raylib's Ray/BoundingBox helpers no
	// longer sit between the gameplay camera and mob targeting.
	rayBoxDistance := func(cx, cy, cz float32, c Collider, maxDistance float32) (float32, bool) {
		tMin, tMax := float32(0), maxDistance
		testAxis := func(origin, direction, minValue, maxValue float32) bool {
			if math.Abs(float64(direction)) < 1e-7 {
				return origin >= minValue && origin <= maxValue
			}
			inv := 1 / direction
			t1 := (minValue - origin) * inv
			t2 := (maxValue - origin) * inv
			if t1 > t2 {
				t1, t2 = t2, t1
			}
			if t1 > tMin {
				tMin = t1
			}
			if t2 < tMax {
				tMax = t2
			}
			return tMin <= tMax
		}
		if !testAxis(rayOrigin.X, rayDirection.X, cx-c.Width/2, cx+c.Width/2) ||
			!testAxis(rayOrigin.Y, rayDirection.Y, cy, cy+c.Height) ||
			!testAxis(rayOrigin.Z, rayDirection.Z, cz-c.Depth/2, cz+c.Depth/2) {
			return 0, false
		}
		if tMin < 0 || tMin > maxDistance {
			return 0, false
		}
		return tMin, true
	}

	for _, e := range remoteEntities {
		if e.MobKind == "" || e.MobHealth <= 0 {
			continue
		}
		d := mobContent.Definitions[e.MobKind]
		distance, rayHit := rayBoxDistance(float32(e.X), float32(e.Y), float32(e.Z), d.Collider, nearest)
		if rayHit && distance < nearest {
			nearest = distance
			mobTarget = e.ID
		}
	}
	if mobTarget != "" && rl.IsMouseButtonDown(rl.MouseLeftButton) {
		state.MiningProgress = 0
		state.MiningTarget = nil
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) && client != nil {
			client.Send(&PacketAttackMob{Target: mobTarget})
		}
		return hit
	}

	// Progressive Mining Logic
	if hit.hit && rl.IsMouseButtonDown(rl.MouseLeftButton) {
		// 1. Get Block Hardness
		blockType := world.BlockAt(hit.x, hit.y, hit.z)
		def := GetBlock(blockType)
		if def.Hardness < 0 && currentGameMode == ModeSurvival {
			state.MiningProgress = 0
			state.MiningTarget = nil
			return hit
		}
		seconds := MiningSeconds(state.Hotbar[state.SelectedSlot], blockType)
		if currentGameMode == ModeCreative {
			if rl.GetTime()-state.LastBreakTime < 0.15 {
				return hit
			}
			seconds = 0
		}

		// 4. Accumulate Progress
		isNewTarget := state.MiningTarget == nil ||
			state.MiningTarget.x != hit.x ||
			state.MiningTarget.y != hit.y ||
			state.MiningTarget.z != hit.z ||
			state.MiningTool != state.Hotbar[state.SelectedSlot] || state.MiningBlock != blockType

		if isNewTarget {
			state.MiningTarget = &hit
			state.MiningTool = state.Hotbar[state.SelectedSlot]
			state.MiningBlock = blockType
			state.MiningProgress = 0
		}

		if seconds == 0 {
			state.MiningProgress = 1
		} else if seconds > 0 {
			state.MiningProgress += gameFrameTime() / seconds
		}

		// 5. Break Block if Done
		if state.MiningProgress >= 1.0 {
			world.RemoveBlock(hit.x, hit.y, hit.z)
			if client != nil {
				client.Send(&PacketBlockChange{
					X:       int32(hit.x),
					Y:       int32(hit.y),
					Z:       int32(hit.z),
					BlockID: blockAir,
				})
			}
			// Reset progress but keep target so we don't instantly break next block unless we click again
			// Actually in MC you keep breaking if you hold.
			// But for safety, let's reset progress to 0.
			// If hardness is low, it will break next one fast too.
			state.MiningProgress = 0
			// Update target to nil so we re-acquire next frame if raycast hits something else
			state.MiningTarget = nil
			state.LastBreakTime = rl.GetTime()
		}
	} else {
		// Not holding button or not hitting block
		state.MiningTarget = nil
		state.MiningProgress = 0
	}

	if hit.hit && rl.IsMouseButtonPressed(rl.MouseRightButton) {
		// 1. Check for Block Interaction (Server Authoritative)
		blockID := world.BlockAt(hit.x, hit.y, hit.z)
		if (blockID == blockCraftingTable || containerSize(blockID) > 0) && !rl.IsKeyDown(rl.KeyLeftShift) {
			if client != nil {
				client.Send(&PacketBlockInteract{
					X:      int32(hit.x),
					Y:      int32(hit.y),
					Z:      int32(hit.z),
					Action: 0,
				})
			}
			return hit // Consume interaction
		}

		if state.CurrentBlock == blockAir || state.CurrentBlock >= 100 {
			return hit
		}
		// 2. Block Placement Logic
		nx := hit.x + int(math.Round(float64(hit.normal.X)))
		ny := hit.y + int(math.Round(float64(hit.normal.Y)))
		nz := hit.z + int(math.Round(float64(hit.normal.Z)))

		canPlace := true
		if isSolidBlock(state.CurrentBlock) {
			if collidesWithBlock(camera.Position, nx, ny, nz) {
				canPlace = false
			}
		}

		if canPlace {
			px, py, pz, ok := world.PlaceAdjacent(hit, state.CurrentBlock)
			if ok && client != nil {
				client.Send(&PacketBlockChange{
					X:       int32(px),
					Y:       int32(py),
					Z:       int32(pz),
					BlockID: state.CurrentBlock,
					Meta:    world.MetaAt(px, py, pz),
				})
			}
		}
	}

	return hit
}

func (s *InputState) UpdateInventoryPage() {
	wheel := rl.GetMouseWheelMove()
	if wheel == 0 {
		return
	}
	layout := inventoryLayout()
	itemsPerPage := layout.Cols * layout.Rows
	totalPages := (len(allBlocks) + itemsPerPage - 1) / itemsPerPage
	if wheel > 0 {
		s.InventoryPage--
	} else if wheel < 0 {
		s.InventoryPage++
	}
	if s.InventoryPage < 0 {
		s.InventoryPage = 0
	}
	if s.InventoryPage >= totalPages {
		s.InventoryPage = totalPages - 1
	}
}

func (s *InputState) UpdateInventorySelection(client *Client) {
	if s.Container != nil {
		s.updateContainerInput()
		return
	}
	// Sync Hotbar Logic (always active)
	if client != nil {
		for i := 0; i < 9; i++ {
			item := client.Inventory.Slots[i]
			if item.ID != 0 {
				s.Hotbar[i] = byte(item.ID)
			} else {
				s.Hotbar[i] = blockAir
			}
		}
		s.CurrentBlock = s.Hotbar[s.SelectedSlot]
	}

	if !s.InventoryOpen {
		return
	}

	// Constants
	scale := inventoryScale()
	mouse := rl.GetMousePosition()
	leftClick := rl.IsMouseButtonPressed(rl.MouseLeftButton)
	rightClick := rl.IsMouseButtonPressed(rl.MouseRightButton)

	// Unified Interaction Handler
	handleSlotInteraction := func(slotIndex int, isCreativeSource bool) {
		button := -1
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			button = 0
		} else if rl.IsMouseButtonPressed(rl.MouseRightButton) {
			button = 1
		}

		if button == -1 {
			return
		}

		if button == 0 && (rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)) && !isCreativeSource {
			button = 2
		}

		// Server Authoritative Mode (Survival)
		if client != nil && !isCreativeSource {
			client.Send(&PacketClickWindow{
				SlotID:     int32(slotIndex),
				Button:     int32(button),
				IsCreative: false,
			})
			return
		}

		// Local / Creative Source Logic
		// If Creative Source, we simulate picking even if connected (client side palette)
		// Or we can send IsCreative=true in packet.
		// For now, let's keep Creative Source local for "Cursor Filling"
		// BUT if we want true server auth, we should send it.
		// Let's keep strict server auth for Survival Inventory.

		if isCreativeSource {
			// Creative Palette Logic (Client Side for now, or send specific packet)
			// Since our PacketClickWindow supports IsCreative, let's try sending it!
			// But the server logic for IsCreative was "TODO".
			// So let's keep Local logic for Creative Source for now to ensure it works.
			if button == 0 { // Left
				if slotIndex >= 0 && slotIndex < len(allBlocks) {
					blockID := allBlocks[slotIndex]
					if blockID != 0 {
						s.CursorItem = Item{ID: int32(blockID), Count: StackLimit(int32(blockID))}
						// If connected, maybe we should tell server we picked this up?
						// Server thinks we have nothing.
						// We need to sync Cursor to server.
						// Existing PacketInventoryUpdate with Slot -1 will do this?
						// But Client -> Server inventory update is "suspicous".
						// For Creative, it's allowed.
						if client != nil {
							client.Send(&PacketInventoryUpdate{
								SlotID: -1,
								ItemID: int32(blockID),
								Count:  StackLimit(int32(blockID)),
							})
						}
					}
				}
			} else if button == 1 { // Right
				if slotIndex >= 0 && slotIndex < len(allBlocks) {
					blockID := allBlocks[slotIndex]
					if blockID != 0 {
						s.CursorItem = Item{ID: int32(blockID), Count: 1}
						if client != nil {
							client.Send(&PacketInventoryUpdate{
								SlotID: -1,
								ItemID: int32(blockID),
								Count:  1,
							})
						}
					}
				}
			}
			return
		}

		localInventory.Click(slotIndex, button, &s.CursorItem)

	}

	// ---- CREATIVE MODE ----
	if currentGameMode == ModeCreative {
		layout := inventoryLayout()
		itemsPerPage := layout.Cols * layout.Rows
		start := s.InventoryPage * itemsPerPage
		slotX := func(col int) float32 { return layout.GridX + float32(col)*layout.Stride }
		slotY := func(row int) float32 { return layout.GridY + float32(row)*layout.Stride }

		for row := 0; row < layout.Rows; row++ {
			for col := 0; col < layout.Cols; col++ {
				index := start + row*layout.Cols + col
				if index >= len(allBlocks) {
					continue
				}
				x := slotX(col)
				y := slotY(row)
				if mouse.X >= x && mouse.X <= x+layout.SlotSize &&
					mouse.Y >= y && mouse.Y <= y+layout.SlotSize {
					handleSlotInteraction(index, true)
				}
			}
		}

		for col := 0; col < layout.Cols; col++ {
			x := layout.HotbarX + float32(col)*layout.Stride
			y := layout.HotbarY
			if mouse.X >= x && mouse.X <= x+layout.SlotSize &&
				mouse.Y >= y && mouse.Y <= y+layout.SlotSize {
				handleSlotInteraction(col, false)
			}
		}

		winW := float32(176) * scale
		winH := float32(196) * scale
		if (leftClick || rightClick) &&
			!rl.CheckCollisionPointRec(mouse, rl.NewRectangle(layout.OriginX, layout.OriginY, winW, winH)) {
			s.CursorItem = Item{} // Drop
			if client != nil {
				client.Send(&PacketInventoryUpdate{SlotID: -1, ItemID: 0, Count: 0})
			}
		}

	} else {
		layout := survivalLayout(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
		for i := 0; i < 36; i++ {
			if rl.CheckCollisionPointRec(mouse, layout.Slot(i)) {
				handleSlotInteraction(i, false)
			}
		}
		s.updateCraftingInput(layout, client)

	}
}

func collidesWithBlock(pos gameVec3, bx, by, bz int) bool {
	feetY := pos.Y - playerEyeY
	minX := pos.X - playerRadius
	maxX := pos.X + playerRadius
	minZ := pos.Z - playerRadius
	maxZ := pos.Z + playerRadius
	minY := feetY
	maxY := feetY + playerHeight

	// Block AABB (Centered)
	// Check intersection (A.min < B.max && A.max > B.min)
	if maxX > float32(bx)-0.5 && minX < float32(bx)+0.5 &&
		maxY > float32(by)-0.5 && minY < float32(by)+0.5 &&
		maxZ > float32(bz)-0.5 && minZ < float32(bz)+0.5 {
		return true
	}
	return false
}
