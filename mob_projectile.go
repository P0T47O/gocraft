package main

import "math"

// Arrows are transient server entities. A segment test prevents a fast arrow
// from tunneling through a player between 20 Hz simulation steps.
type ArrowEntity struct {
	BaseEntity
	Owner      string
	Vx, Vy, Vz float64
	Damage     int
	Age        int
	Dead       bool
	HitPlayer  *PlayerEntity
}

func (a *ArrowEntity) Tick(w *World) {
	if a.Dead {
		return
	}
	a.Age++
	if a.Age > 80 {
		a.Dead = true
		return
	}
	a.Vy -= .018
	distance := math.Sqrt(a.Vx*a.Vx + a.Vy*a.Vy + a.Vz*a.Vz)
	if distance < .001 {
		a.Dead = true
		return
	}
	startX, startY, startZ := a.X, a.Y, a.Z
	hit := w.rayCast(float32(a.X), float32(a.Y), float32(a.Z), float32(a.Vx/distance), float32(a.Vy/distance), float32(a.Vz/distance), float32(distance))
	travel := distance
	if hit.hit {
		travel = math.Min(travel, float64(hit.distance))
	}
	for _, e := range w.entities {
		p, ok := e.(*PlayerEntity)
		if !ok || p.UUID == a.Owner || p.dead() || p.GameMode != ModeSurvival {
			continue
		}
		// Closest approach to the player's torso along the traveled segment.
		px, py, pz := p.X-startX, p.Y-playerEyeY*.55-startY, p.Z-startZ
		projection := (px*a.Vx + py*a.Vy + pz*a.Vz) / distance
		if projection < 0 || projection > travel {
			continue
		}
		dx, dy, dz := px-a.Vx/distance*projection, py-a.Vy/distance*projection, pz-a.Vz/distance*projection
		if dx*dx+dy*dy+dz*dz < .55*.55 {
			a.HitPlayer, a.Dead = p, true
			return
		}
	}
	if hit.hit {
		a.Dead = true
		return
	}
	a.X += a.Vx
	a.Y += a.Vy
	a.Z += a.Vz
	a.Yaw = float32(math.Atan2(a.Vx, a.Vz))
	a.Dirty = true
}
