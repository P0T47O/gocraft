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
	CurrentIndex  int
	CurrentBlock  byte
	InventoryOpen bool
	SelectedSlot  int
	Hotbar        [9]byte
	SkipCamera    bool
	Yaw           float32
	Pitch         float32
	Sensitivity   float32
	MoveSpeed     float32
	CursorItem    Item // Held item on mouse cursor
	InventoryPage int
	ShowDebug     bool

	// Survival Mode Physics
	VelocityY float32 // Vertical velocity
	OnGround  bool    // Is player standing on solid ground
	LastWTime float64 // Time of last W key press (for double-tap detection)
	IsRunning bool    // Running state (activated by double-tap W)

	// Mining System
	MiningTarget   *hitInfo // Block currently being mined (can be nil)
	MiningProgress float32  // 0.0 to 1.0
	LastMiningTime float64  // Time of last frame's mining logic
	LastBreakTime  float64  // Time of last block break (Creative delay)
}

// Block hardness values (seconds to break with hand/wrong tool)
// We'll calculate speed based on tool later, for now this is base time
var BlockHardness = map[byte]float32{
	blockDirt:        0.75,
	blockGrass:       0.9,
	blockStone:       4.0, // Hard to break by hand
	blockLog:         3.0,
	blockLeaves:      0.3,
	blockSand:        0.6,
	blockGlass:       0.4,
	blockPlank:       2.0,
	blockCobblestone: 3.5,
	blockTorch:       0.0, // Instabreak
	blockCactus:      0.6,
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

func (s *InputState) InitFromCamera(camera rl.Camera3D) {
	dir := rl.Vector3Subtract(camera.Target, camera.Position)
	dir = rl.Vector3Normalize(dir)
	s.Yaw = float32(math.Atan2(float64(dir.X), float64(dir.Z)))
	s.Pitch = float32(math.Asin(float64(dir.Y)))
}

func (s *InputState) ToggleInventory() {
	if rl.IsKeyPressed(rl.KeyE) {
		s.InventoryOpen = !s.InventoryOpen
		s.SkipCamera = true
		if s.InventoryOpen {
			rl.EnableCursor()
			rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
		} else {
			rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
			rl.DisableCursor()
		}
	}
	if s.InventoryOpen && rl.IsKeyPressed(rl.KeyEscape) {
		s.InventoryOpen = false
		s.SkipCamera = true

		rl.SetMousePosition(int32(rl.GetScreenWidth()/2), int32(rl.GetScreenHeight()/2))
		rl.DisableCursor()
	}
}

func (s *InputState) UpdateCamera(world *World, camera *rl.Camera3D) {
	delta := rl.GetMouseDelta()
	s.Yaw -= delta.X * s.Sensitivity
	s.Pitch -= delta.Y * s.Sensitivity
	if s.Pitch > 1.55 {
		s.Pitch = 1.55
	} else if s.Pitch < -1.55 {
		s.Pitch = -1.55
	}

	forward := rl.NewVector3(
		float32(math.Sin(float64(s.Yaw)))*float32(math.Cos(float64(s.Pitch))),
		float32(math.Sin(float64(s.Pitch))),
		float32(math.Cos(float64(s.Yaw)))*float32(math.Cos(float64(s.Pitch))),
	)
	forward = rl.Vector3Normalize(forward)
	up := rl.NewVector3(0, 1, 0)

	flatForward := rl.NewVector3(forward.X, 0, forward.Z)
	if rl.Vector3Length(flatForward) > 0 {
		flatForward = rl.Vector3Normalize(flatForward)
	}
	right := rl.Vector3CrossProduct(flatForward, up)
	if rl.Vector3Length(right) > 0 {
		right = rl.Vector3Normalize(right)
	}

	dt := rl.GetFrameTime()

	// Creative Mode: Free flying
	if currentGameMode == ModeCreative {
		move := rl.NewVector3(0, 0, 0)
		if rl.IsKeyDown(rl.KeyW) {
			move = rl.Vector3Add(move, flatForward)
		}
		if rl.IsKeyDown(rl.KeyS) {
			move = rl.Vector3Subtract(move, flatForward)
		}
		if rl.IsKeyDown(rl.KeyD) {
			move = rl.Vector3Add(move, right)
		}
		if rl.IsKeyDown(rl.KeyA) {
			move = rl.Vector3Subtract(move, right)
		}
		if rl.IsKeyDown(rl.KeySpace) {
			move.Y += 1
		}
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			move.Y -= 1
		}
		if rl.Vector3Length(move) > 0 {
			move = rl.Vector3Normalize(move)
		}
		camera.Position = resolveCollision(world, camera.Position, rl.Vector3Scale(move, flySpeed*dt))
	} else {
		// Survival Mode: Gravity-based movement

		// Double-tap W detection for running
		currentTime := rl.GetTime()
		if rl.IsKeyPressed(rl.KeyW) {
			if currentTime-s.LastWTime < doubleTapTime {
				s.IsRunning = true
			}
			s.LastWTime = currentTime
		}
		// Stop running when W is released
		if !rl.IsKeyDown(rl.KeyW) {
			s.IsRunning = false
		}

		// Horizontal movement
		move := rl.NewVector3(0, 0, 0)
		if rl.IsKeyDown(rl.KeyW) {
			move = rl.Vector3Add(move, flatForward)
		}
		if rl.IsKeyDown(rl.KeyS) {
			move = rl.Vector3Subtract(move, flatForward)
		}
		if rl.IsKeyDown(rl.KeyD) {
			move = rl.Vector3Add(move, right)
		}
		if rl.IsKeyDown(rl.KeyA) {
			move = rl.Vector3Subtract(move, right)
		}
		if rl.Vector3Length(move) > 0 {
			move = rl.Vector3Normalize(move)
		}

		// Determine movement speed
		speed := float32(walkSpeed)
		if s.IsRunning {
			speed = runSpeed
		}

		// Apply horizontal movement with collision
		newPos := resolveCollision(world, camera.Position, rl.Vector3Scale(move, speed*dt))

		// Apply gravity (always, unless jumping this frame)
		s.VelocityY -= gravity * dt
		if s.VelocityY < -terminalVel {
			s.VelocityY = -terminalVel
		}

		// Jump input (must be before vertical collision so jump can happen)
		if s.OnGround && rl.IsKeyPressed(rl.KeySpace) {
			s.VelocityY = jumpVelocity
		}

		// Calculate vertical movement
		verticalDelta := s.VelocityY * dt

		// Apply vertical movement with collision
		posBeforeVertical := newPos
		newPos = resolveCollision(world, newPos, rl.NewVector3(0, verticalDelta, 0))

		// Minecraft-style ground detection:
		// If we tried to move down but couldn't (or moved less), we're on ground
		actualVerticalMove := newPos.Y - posBeforeVertical.Y

		if verticalDelta < 0 {
			// Was trying to fall
			if actualVerticalMove > verticalDelta+0.001 {
				// Collision stopped us from falling as much as we wanted = on ground
				s.OnGround = true
				s.VelocityY = 0
			} else {
				s.OnGround = false
			}
		} else if verticalDelta > 0 {
			// Was trying to jump/rise
			if actualVerticalMove < verticalDelta-0.001 {
				// Hit ceiling
				s.VelocityY = 0
			}
			s.OnGround = false
		} else {
			// No vertical movement requested, check if we should start falling
			// Try a tiny downward probe
			probePos := resolveCollision(world, newPos, rl.NewVector3(0, -0.01, 0))
			if probePos.Y < newPos.Y-0.005 {
				// We can fall, so we're not on ground
				s.OnGround = false
			}
			// If can't fall, stay at current OnGround state
		}

		camera.Position = newPos
	}

	camera.Target = rl.Vector3Add(camera.Position, forward)
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

func (s *InputState) RayFromCenter(camera rl.Camera3D) rl.Ray {
	center := rl.NewVector2(float32(rl.GetScreenWidth()/2), float32(rl.GetScreenHeight()/2))
	return rl.GetMouseRay(center, camera)
}

func resolveCollision(world *World, pos rl.Vector3, delta rl.Vector3) rl.Vector3 {
	next := pos
	if delta.X != 0 {
		test := rl.NewVector3(next.X+delta.X, next.Y, next.Z)
		if !collides(world, test) {
			next = test
		}
	}
	if delta.Z != 0 {
		test := rl.NewVector3(next.X, next.Y, next.Z+delta.Z)
		if !collides(world, test) {
			next = test
		}
	}
	if delta.Y != 0 {
		test := rl.NewVector3(next.X, next.Y+delta.Y, next.Z)
		if !collides(world, test) {
			next = test
		}
	}
	return next
}

func collides(world *World, pos rl.Vector3) bool {
	feetY := pos.Y - playerEyeY
	minX := pos.X - playerRadius - 0.001
	maxX := pos.X + playerRadius - 0.001
	minZ := pos.Z - playerRadius - 0.001
	maxZ := pos.Z + playerRadius + 0.001
	minY := feetY
	maxY := feetY + playerHeight

	minBX := blockIndexFromCoord(minX)
	maxBX := blockIndexFromCoord(maxX)
	minBZ := blockIndexFromCoord(minZ)
	maxBZ := blockIndexFromCoord(maxZ)
	minBY := blockIndexFromCoord(minY)
	maxBY := blockIndexFromCoord(maxY)

	for x := minBX; x <= maxBX; x++ {
		for y := minBY; y <= maxBY; y++ {
			for z := minBZ; z <= maxBZ; z++ {
				if isSolidBlock(world.BlockAt(x, y, z)) {
					// Precise AABB check for centered blocks
					bx, by, bz := float32(x), float32(y), float32(z)
					if maxX > bx-0.5 && minX < bx+0.5 &&
						maxY > by-0.5 && minY < by+0.5 &&
						maxZ > bz-0.5 && minZ < bz+0.5 {
						return true
					}
				}
			}
		}
	}
	return false
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

func HandleInput(world *World, camera *rl.Camera3D, state *InputState, client *Client) hitInfo {
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

	ray := state.RayFromCenter(*camera)

	// Reach distance depends on Gamemode
	reachDist := float32(8.0) // Creative default
	if currentGameMode == ModeSurvival {
		reachDist = 3.0
	}

	hit := world.HitTest(ray, reachDist)

	// Progressive Mining Logic
	if hit.hit && rl.IsMouseButtonDown(rl.MouseLeftButton) {
		// 1. Get Block Hardness
		blockType := world.BlockAt(hit.x, hit.y, hit.z)
		hardness := float32(1.0) // Default hardness
		if h, ok := BlockHardness[blockType]; ok {
			hardness = h
		}

		// Creative Mode Instabreak with delay
		if currentGameMode == ModeCreative {
			if rl.GetTime()-state.LastBreakTime < 0.15 {
				hardness = 100000.0 // Prevent break
			} else {
				hardness = 0.0
			}
		}

		// 2. Check tool speed
		heldItem := state.Hotbar[state.SelectedSlot]
		speedMultiplier := GetMiningSpeedMultiplier(heldItem, blockType)

		// 3. Accumulate Progress
		isNewTarget := state.MiningTarget == nil ||
			state.MiningTarget.x != hit.x ||
			state.MiningTarget.y != hit.y ||
			state.MiningTarget.z != hit.z

		if isNewTarget {
			state.MiningTarget = &hit
			state.MiningProgress = 0
		}

		// Add progress
		if hardness <= 0 {
			state.MiningProgress = 1.0 // Instabreak
		} else {
			// Effective hardness = Base Hardness / Speed Multiplier
			state.MiningProgress += rl.GetFrameTime() * speedMultiplier / hardness
		}

		// 4. Break Block if Done
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
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
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
						s.CursorItem = Item{ID: int32(blockID), Count: MaxStackSize}
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
								Count:  MaxStackSize,
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

		// Offline Survival Logic (Fallback)
		currentInventory := &localInventory
		if slotIndex < 0 || slotIndex >= len(currentInventory.Slots) {
			return
		}
		targetSlot := &currentInventory.Slots[slotIndex]
		cursor := &s.CursorItem

		if button == 0 { // Left Click
			if cursor.ID == 0 {
				if targetSlot.ID != 0 {
					*cursor = *targetSlot
					*targetSlot = Item{}
				}
			} else {
				if targetSlot.ID == 0 {
					*targetSlot = *cursor
					*cursor = Item{}
				} else if targetSlot.ID == cursor.ID {
					space := int32(MaxStackSize) - targetSlot.Count
					if space > 0 {
						toAdd := cursor.Count
						if toAdd > space {
							toAdd = space
						}
						targetSlot.Count += toAdd
						cursor.Count -= toAdd
						if cursor.Count == 0 {
							cursor.ID = 0
						}
					}
				} else {
					tmp := *cursor
					*cursor = *targetSlot
					*targetSlot = tmp
				}
			}
		} else if button == 1 { // Right Click
			if cursor.ID == 0 {
				if targetSlot.ID != 0 {
					split := int32(math.Ceil(float64(targetSlot.Count) / 2.0))
					*cursor = Item{ID: targetSlot.ID, Count: split}
					targetSlot.Count -= split
					if targetSlot.Count == 0 {
						targetSlot.ID = 0
					}
				}
			} else {
				if targetSlot.ID == 0 {
					*targetSlot = Item{ID: cursor.ID, Count: 1}
					cursor.Count--
				} else if targetSlot.ID == cursor.ID {
					if targetSlot.Count < MaxStackSize {
						targetSlot.Count++
						cursor.Count--
					}
				}
				if cursor.Count == 0 {
					cursor.ID = 0
				}
			}
		}
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
		// ---- SURVIVAL MODE ----
		slotSize := 36 * scale / 2
		if slotSize < 32 {
			slotSize = 32
		}
		stride := slotSize + 4
		cols := 9
		rows := 3

		invW := float32(cols)*stride + 20
		invH := float32(rows+1)*stride + 60
		startX := (w - invW) / 2
		startY := (h - invH) / 2
		hotbarY := startY + invH - stride - 10
		mainY := startY + 40

		checkAndHandle := func(slotIndex int, x, y float32) {
			if mouse.X >= x && mouse.X <= x+slotSize &&
				mouse.Y >= y && mouse.Y <= y+slotSize {
				handleSlotInteraction(slotIndex, false)
			}
		}

		for i := 0; i < 9; i++ {
			x := startX + 10 + float32(i)*stride
			checkAndHandle(i, x, hotbarY)
		}
		for i := 9; i < 36; i++ {
			idx := i - 9
			r := idx / 9
			c := idx % 9
			x := startX + 10 + float32(c)*stride
			y := mainY + float32(r)*stride
			checkAndHandle(i, x, y)
		}

		if (leftClick || rightClick) &&
			!rl.CheckCollisionPointRec(mouse, rl.NewRectangle(startX, startY, invW, invH)) {
			s.CursorItem = Item{}
			if client != nil {
				client.Send(&PacketInventoryUpdate{SlotID: -1, ItemID: 0, Count: 0})
			}
		}
	}
}

func collidesWithBlock(pos rl.Vector3, bx, by, bz int) bool {
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
