package main

import (
	"math"
)

func updateEntities(dt float32) {
	tickExplosionEffects(dt)
}

func updateInterpolation(dt float32) {
	for _, e := range remoteEntities {
		lerpFactor := float64(dt * 10.0)
		if lerpFactor > 1.0 {
			lerpFactor = 1.0
		}
		oldX, oldY, oldZ := e.X, e.Y, e.Z
		e.X += (e.TX - e.X) * lerpFactor
		e.Y += (e.TY - e.Y) * lerpFactor
		e.Z += (e.TZ - e.Z) * lerpFactor
		if d, ok := mobContent.Definitions[e.MobKind]; ok {
			anim := mobContent.Animations[d.Animation]
			distance := float32(math.Hypot(e.X-oldX, e.Z-oldZ))
			if e.MobState == "climb" {
				distance += float32(math.Abs(e.Y - oldY))
			}
			if distance < 2 {
				e.AnimPhase += distance / anim.Stride * 2 * math.Pi
			}
			target := float32(0)
			if e.MobState == "walk" || e.MobState == "flee" || e.MobState == "chase" || e.MobState == "climb" {
				target = 1
			}
			e.AnimBlend += (target - e.AnimBlend) * min(dt*anim.BlendSpeed, float32(1))
			// Passive mobs can change course abruptly at a wall or a ledge.
			// Position and yaw are interpolated independently, so using the
			// server yaw while the old position catches up makes them slide
			// sideways or even walk backwards. Face the visible motion while
			// walking; keep the server yaw for idle and hostile aiming.
			if !d.Hostile && (e.MobState == "walk" || e.MobState == "flee") && distance > .0001 {
				e.Yaw = float32(math.Atan2(e.X-oldX, e.Z-oldZ))
			} else {
				diff := float32(math.Atan2(math.Sin(float64(e.TargetYaw-e.Yaw)), math.Cos(float64(e.TargetYaw-e.Yaw))))
				e.Yaw += diff * float32(lerpFactor)
			}
			e.MobHurt = max(0, e.MobHurt-dt)
			if e.MobHealth <= 0 {
				e.DeathTime += dt
			}
		}
	}
}
