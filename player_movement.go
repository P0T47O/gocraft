package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
)

type MovementControls struct {
	Forward, Side       float32
	Jump, Sneak, Sprint bool
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
func offsetAxis(p rl.Vector3, axis int, d float32) rl.Vector3 {
	switch axis {
	case 0:
		p.X += d
	case 1:
		p.Y += d
	case 2:
		p.Z += d
	}
	return p
}
func blockAtPosition(w *World, x, y, z float64) byte {
	return w.BlockAt(int(math.Floor(x+0.5)), int(math.Floor(y+0.5)), int(math.Floor(z+0.5)))
}
func feetSupported(w *World, p rl.Vector3) bool { return collides(w, offsetAxis(p, 1, -0.06)) }
func touchesLiquid(w *World, p rl.Vector3, id byte) bool {
	for _, dy := range []float32{-playerEyeY + 0.1, -0.8, 0} {
		if blockAtPosition(w, float64(p.X), float64(p.Y+dy), float64(p.Z)) == id {
			return true
		}
	}
	return false
}

func (s *InputState) StepMovement(w *World, p rl.Vector3, dt float32, c MovementControls, creative bool) rl.Vector3 {
	dt = min(max(dt, 0), 0.1)
	s.IsSneaking = c.Sneak && !creative
	s.IsSwimming = !creative && touchesLiquid(w, p, blockWater)
	s.IsRunning = !creative && !s.IsSwimming && !c.Sneak && c.Sprint && c.Forward > 0
	sn, cs := float32(math.Sin(float64(s.Yaw))), float32(math.Cos(float64(s.Yaw)))
	move := rl.NewVector3(sn*c.Forward-cs*c.Side, 0, cs*c.Forward+sn*c.Side)
	if rl.Vector3Length(move) > 1 {
		move = rl.Vector3Normalize(move)
	}
	if creative {
		s.VelocityY = 0
		s.OnGround = false
		if c.Jump {
			move.Y++
		}
		if c.Sneak {
			move.Y--
		}
		if rl.Vector3Length(move) > 1 {
			move = rl.Vector3Normalize(move)
		}
		return resolveCollision(w, p, rl.Vector3Scale(move, flySpeed*dt))
	}
	speed := float32(walkSpeed)
	if s.IsRunning {
		speed = runSpeed
	}
	if s.IsSneaking {
		speed = 1.3
	}
	if s.IsSwimming {
		speed = 2.3
	}
	remaining := dt
	for remaining > 0 {
		step := min(remaining, float32(1.0/60))
		remaining -= step
		grounded := feetSupported(w, p)
		wet := touchesLiquid(w, p, blockWater)
		next := resolveCollision(w, p, rl.Vector3Scale(move, speed*step))
		if c.Sneak && grounded && !wet && !c.Jump {
			if !feetSupported(w, rl.NewVector3(next.X, p.Y, p.Z)) {
				next.X = p.X
			}
			if !feetSupported(w, rl.NewVector3(next.X, p.Y, next.Z)) {
				next.Z = p.Z
			}
		}
		if wet {
			// Drag cancels fall momentum on entering water, including from a cliff.
			s.VelocityY = max(float32(-2), s.VelocityY)
			target := float32(-0.5)
			if c.Jump {
				target = 3.4
			}
			if c.Sneak {
				target = -3.0
			}
			s.VelocityY += (target - s.VelocityY) * min(step*10, 1)
			if c.Jump && blockAtPosition(w, float64(p.X), float64(p.Y), float64(p.Z)) == blockAir &&
				(abs32(next.X-p.X) < abs32(move.X*speed*step)*0.5 || abs32(next.Z-p.Z) < abs32(move.Z*speed*step)*0.5) {
				s.VelocityY = jumpVelocity
			}
		} else {
			if grounded && s.VelocityY < 0 {
				s.VelocityY = 0
			}
			if grounded && c.Jump {
				s.VelocityY = jumpVelocity
			}
			s.VelocityY = max(s.VelocityY-gravity*step, float32(-terminalVel))
		}
		before := next.Y
		next = resolveCollision(w, next, rl.NewVector3(0, s.VelocityY*step, 0))
		if abs32((next.Y-before)-s.VelocityY*step) > 0.001 {
			s.VelocityY = 0
		}
		p = next
		s.OnGround = feetSupported(w, p)
	}
	return p
}
