package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	playerRadius = 0.3
	playerHeight = 1.8
	playerEyeY   = 1.62
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
	Dragging      bool
	DragBlock     byte
	InventoryPage int
	ShowDebug     bool
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
		s.Dragging = false
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

	camera.Position = resolveCollision(world, camera.Position, rl.Vector3Scale(move, s.MoveSpeed*dt))
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
	}
	for i := 0; i < 9; i++ {
		if rl.IsKeyPressed(int32(rl.KeyOne + int32(i))) {
			if i < len(s.Hotbar) {
				s.SelectedSlot = i
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
	maxX := pos.X + playerRadius + 0.001
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
					return true
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
	state.ToggleInventory()
	if state.SkipCamera {
		rl.GetMouseDelta()
		state.SkipCamera = false
	}
	if state.InventoryOpen {
		state.UpdateSelection(false)
		state.UpdateInventoryPage()
		state.UpdateInventorySelection()
		return hitInfo{}
	}

	state.UpdateCamera(world, camera)
	state.UpdateSelection(true)

	ray := state.RayFromCenter(*camera)
	hit := world.HitTest(ray)

	if hit.hit && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		world.RemoveBlock(hit.x, hit.y, hit.z)
		if client != nil {
			client.Send(&PacketBlockChange{
				X:       int32(hit.x),
				Y:       int32(hit.y),
				Z:       int32(hit.z),
				BlockID: blockAir,
			})
		}
	}

	if hit.hit && rl.IsMouseButtonPressed(rl.MouseRightButton) {
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

func (s *InputState) UpdateInventorySelection() {
	if !rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		return
	}
	layout := inventoryLayout()
	mouse := rl.GetMousePosition()

	slotX := func(col int) float32 {
		return layout.GridX + float32(col)*layout.Stride
	}
	slotY := func(row int) float32 {
		return layout.GridY + float32(row)*layout.Stride
	}

	itemsPerPage := layout.Cols * layout.Rows
	start := s.InventoryPage * itemsPerPage

	if s.Dragging {
		for col := 0; col < layout.Cols; col++ {
			x := layout.HotbarX + float32(col)*layout.Stride
			y := layout.HotbarY
			if mouse.X >= x && mouse.X <= x+layout.SlotSize &&
				mouse.Y >= y && mouse.Y <= y+layout.SlotSize {
				s.Hotbar[col] = s.DragBlock
				s.SelectedSlot = col
				s.CurrentBlock = s.Hotbar[s.SelectedSlot]
				s.Dragging = false
				return
			}
		}
		return
	}

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
				s.Dragging = true
				s.DragBlock = allBlocks[index]
				return
			}
		}
	}

	for col := 0; col < layout.Cols; col++ {
		x := layout.HotbarX + float32(col)*layout.Stride
		y := layout.HotbarY
		if mouse.X >= x && mouse.X <= x+layout.SlotSize &&
			mouse.Y >= y && mouse.Y <= y+layout.SlotSize {
			s.SelectedSlot = col
			s.CurrentBlock = s.Hotbar[s.SelectedSlot]
			return
		}
	}
}
