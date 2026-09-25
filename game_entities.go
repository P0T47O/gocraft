package main

import (
	"math"
)

func updateEntities(dt float32) {
	// Not much to do here for now besides interpolation handled below
}

func updateInterpolation(dt float32) {
	for _, e := range remoteEntities {
		lerpFactor := float64(dt * 10.0)
		if lerpFactor > 1.0 {
			lerpFactor = 1.0
		}
		oldX, oldZ := e.X, e.Z
		e.X += (e.TX - e.X) * lerpFactor
		e.Y += (e.TY - e.Y) * lerpFactor
		e.Z += (e.TZ - e.Z) * lerpFactor
		if d, ok := mobContent.Definitions[e.MobKind]; ok {
			anim := mobContent.Animations[d.Animation]
			distance := float32(math.Hypot(e.X-oldX, e.Z-oldZ))
			if distance < 2 {
				e.AnimPhase += distance / anim.Stride * 2 * math.Pi
			}
			target := float32(0)
			if e.MobState == "walk" || e.MobState == "flee" || e.MobState == "chase" {
				target = 1
			}
			e.AnimBlend += (target - e.AnimBlend) * min(dt*anim.BlendSpeed, float32(1))
			diff := float32(math.Atan2(math.Sin(float64(e.TargetYaw-e.Yaw)), math.Cos(float64(e.TargetYaw-e.Yaw))))
			e.Yaw += diff * float32(lerpFactor)
			e.MobHurt = max(0, e.MobHurt-dt)
			if e.MobHealth <= 0 {
				e.DeathTime += dt
			}
		}
	}
}
