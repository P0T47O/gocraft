package main

import (
	"math"
)

type EntityType int

const (
	EntityPlayer EntityType = iota
	EntityPig
	EntityItem
)

type Entity interface {
	GetUUID() string
	GetType() EntityType
	GetPosition() (float64, float64, float64)
	SetPosition(x, y, z float64)
	GetRotation() (float32, float32)
	SetRotation(yaw, pitch float32)
	Tick(world *World)
	IsDirty() bool
	ClearDirty()
}

type BaseEntity struct {
	UUID       string
	Type       EntityType
	X, Y, Z    float64
	Yaw, Pitch float32
	Dirty      bool
}

func (e *BaseEntity) GetUUID() string                          { return e.UUID }
func (e *BaseEntity) GetType() EntityType                      { return e.Type }
func (e *BaseEntity) GetPosition() (float64, float64, float64) { return e.X, e.Y, e.Z }
func (e *BaseEntity) SetPosition(x, y, z float64)              { e.X, e.Y, e.Z = x, y, z; e.Dirty = true }
func (e *BaseEntity) GetRotation() (float32, float32)          { return e.Yaw, e.Pitch }
func (e *BaseEntity) SetRotation(yaw, pitch float32)           { e.Yaw, e.Pitch = yaw, pitch; e.Dirty = true }
func (e *BaseEntity) IsDirty() bool                            { return e.Dirty }
func (e *BaseEntity) ClearDirty()                              { e.Dirty = false }
func (e *BaseEntity) Tick(world *World)                        {} // Default empty tick

type PigEntity struct {
	BaseEntity
	moveTimer float32
}

func (p *PigEntity) Tick(world *World) {
	// Simple random movement logic
	p.moveTimer -= 0.05
	if p.moveTimer <= 0 {
		p.moveTimer = 2.0 + float32(p.X*7+p.Z*13)  // Deterministic pseudo-random based on position
		if p.moveTimer < 2.0 {
			p.moveTimer = 2.0
		} else if p.moveTimer > 4.0 {
			p.moveTimer = 4.0
		}
		// Change rotation using a simple hash of position instead of time.Now()
		p.Yaw += float32(int(p.X*31+p.Z*17)%360) - 180.0
		p.Dirty = true
	}

	// Move forward based on Yaw
	rad := float64(p.Yaw) * math.Pi / 180.0
	dx := math.Sin(rad) * 0.05
	dz := math.Cos(rad) * 0.05

	p.X += dx
	p.Z += dz

	// Basic Gravity (keep it on surface for now)
	h := float64(world.HeightAt(int(p.X), int(p.Z)))
	if p.Y > h {
		p.Y -= 0.1
		if p.Y < h {
			p.Y = h
		}
	} else if p.Y < h {
		p.Y = h
	}
	p.Dirty = true
}

type ItemEntity struct {
	BaseEntity
	ItemID      byte
	Count       int
	Vx, Vy, Vz  float64
	PickupDelay float32 // Seconds
	Age         float32 // Seconds
	Dead        bool    // Marked for removal after merge
}

func (i *ItemEntity) Tick(world *World) {
	if i.Dead {
		return
	}

	// Gravity
	i.Vy -= 0.04
	// Terminal velocity
	if i.Vy < -1.0 {
		i.Vy = -1.0
	}

	// Move
	i.X += i.Vx
	i.Y += i.Vy
	i.Z += i.Vz

	// Friction
	i.Vx *= 0.91
	i.Vz *= 0.91

	// Collision with Ground
	radius := 0.2
	feetY := i.Y - radius
	bx, by, bz := int(math.Floor(i.X)), int(math.Floor(feetY)), int(math.Floor(i.Z))

	blockID := world.BlockAt(bx, by, bz)
	def := GetBlock(blockID)

	if def.IsCollidable {
		// Snap to top of block
		groundHeight := float64(by) + 1.0
		if feetY < groundHeight {
			i.Y = groundHeight + radius
			i.Vy = 0
			i.Vx *= 0.6 // Ground friction
			i.Vz *= 0.6
		}
	}

	// Age & Pickup Delay
	i.PickupDelay -= 0.05 // Assuming 20 TPS
	i.Age += 0.05

	// Auto-despawn after 5 minutes (300 seconds)
	if i.Age >= 300.0 {
		i.Dead = true
		return
	}

	// Merge with nearby items (only when on ground and after pickup delay)
	// OPTIMIZE: O(N^2) complexity where N is entities in world. Use spatial partition (chunk-based lists).
	if i.PickupDelay <= 0 && i.Vy == 0 && i.Count < MaxStackSize {
		for _, e := range world.entities {
			other, ok := e.(*ItemEntity)
			if !ok || other == i || other.Dead {
				continue
			}
			if other.ItemID != i.ItemID {
				continue
			}
			// Distance check (radius 1 block)
			dx := other.X - i.X
			dz := other.Z - i.Z
			dy := other.Y - i.Y
			distSq := dx*dx + dy*dy + dz*dz
			if distSq < 1.0 { // Within 1 block
				// Merge
				space := MaxStackSize - i.Count
				if other.Count <= space {
					i.Count += other.Count
					other.Dead = true
				} else {
					i.Count = MaxStackSize
					other.Count -= space
				}
				i.Dirty = true
				break // Only merge one per tick
			}
		}
	}

	i.Dirty = true
}

// GameMode definitions
const (
	ModeCreative = 0
	ModeSurvival = 1
)

type PlayerEntity struct {
	BaseEntity
	GameMode     byte
	Inventory    Inventory
	SelectedSlot int
	CursorItem   Item // Held on mouse
}

func (p *PlayerEntity) Tick(world *World) {
	// Sync logic...
}
