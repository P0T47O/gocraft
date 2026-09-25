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
		// Test the traveled segment against the whole player body. A torso
		// sphere missed otherwise valid head and leg hits.
		feet := p.Y - playerEyeY
		if segmentHitsBox(startX, startY, startZ, a.Vx, a.Vy, a.Vz, travel/distance,
			p.X-.3, feet, p.Z-.3, p.X+.3, feet+1.8, p.Z+.3) {
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

func segmentHitsBox(x, y, z, dx, dy, dz, maxT, minX, minY, minZ, maxX, maxY, maxZ float64) bool {
	t0, t1 := 0.0, maxT
	for _, axis := range [3][4]float64{{x, dx, minX, maxX}, {y, dy, minY, maxY}, {z, dz, minZ, maxZ}} {
		if math.Abs(axis[1]) < 1e-9 {
			if axis[0] < axis[2] || axis[0] > axis[3] {
				return false
			}
			continue
		}
		near, far := (axis[2]-axis[0])/axis[1], (axis[3]-axis[0])/axis[1]
		if near > far {
			near, far = far, near
		}
		t0, t1 = math.Max(t0, near), math.Min(t1, far)
		if t0 > t1 {
			return false
		}
	}
	return true
}
