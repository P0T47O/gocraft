package main

import "flag"

var mobPreview = flag.Bool("mob-preview", false, "Preview mobs in WebGPU: 1-5 animation, R reload, B collider")
var mobWorkshopActive, mobWorkshopBounds bool

func advanceMobPreview(e *RemoteEntity, content *MobContent, mode string, dt float32) {
	d := content.Definitions[e.MobKind]
	anim := content.Animations[d.Animation]
	speed := float32(0)
	if mode == "walk" {
		speed = d.Speed
	}
	if mode == "flee" {
		speed = d.FleeSpeed
	}
	target := float32(0)
	if speed > 0 {
		target = 1
	}
	e.AnimBlend += (target - e.AnimBlend) * min(dt*anim.BlendSpeed, float32(1))
	e.AnimPhase += speed * dt / anim.Stride * 6.283185
	e.MobHurt = 0
	if mode == "hurt" {
		e.MobHurt = 1
	}
	if mode == "dead" {
		e.DeathTime += dt
	} else {
		e.DeathTime = 0
	}
}
