package main

import (
	"flag"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

var mobPreview = flag.Bool("mob-preview", false, "Preview mob models: 1-5 animation, R reload, B collider")

func runMobPreview() {
	rl.InitWindow(1100, 760, "Mob workshop")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	content := mobContent
	if local, err := reloadMobPreview(); err == nil {
		content = local
	}
	renderer, err := newMobRenderer(content)
	if err != nil {
		panic(err)
	}
	defer func() { renderer.Close() }()
	e := &RemoteEntity{MobKind: "pig", MobHealth: 10}
	mode := "idle"
	debug := true
	status := "1 Idle   2 Walk   3 Flee   4 Hurt   5 Death   R Reload   B Bounds   A/D Rotate"
	for !rl.WindowShouldClose() {
		dt := rl.GetFrameTime()
		if rl.IsKeyPressed(rl.KeyOne) {
			mode = "idle"
		}
		if rl.IsKeyPressed(rl.KeyTwo) {
			mode = "walk"
		}
		if rl.IsKeyPressed(rl.KeyThree) {
			mode = "flee"
		}
		if rl.IsKeyPressed(rl.KeyFour) {
			mode = "hurt"
		}
		if rl.IsKeyPressed(rl.KeyFive) {
			mode = "dead"
			e.DeathTime = 0
		}
		if rl.IsKeyDown(rl.KeyA) {
			e.Yaw -= dt
		}
		if rl.IsKeyDown(rl.KeyD) {
			e.Yaw += dt
		}
		if rl.IsKeyPressed(rl.KeyB) {
			debug = !debug
		}
		if rl.IsKeyPressed(rl.KeyR) {
			next, loadErr := reloadMobPreview()
			if loadErr == nil {
				var replacement *MobRenderer
				replacement, loadErr = newMobRenderer(next)
				if loadErr == nil {
					renderer.Close()
					renderer = replacement
					content = next
				}
			}
			if loadErr != nil {
				status = loadErr.Error()
			} else {
				status = "Reloaded definitions, model and animations"
			}
		}
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
		cam := rl.Camera3D{Position: rl.NewVector3(2.4, 1.8, 3), Target: rl.NewVector3(0, .5, 0), Up: rl.NewVector3(0, 1, 0), Fovy: 45}
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(22, 32, 35, 255))
		rl.BeginMode3D(cam)
		rl.DrawGrid(12, 1)
		renderer.Draw(e, debug)
		rl.EndMode3D()
		rl.DrawText("MOB WORKSHOP", 28, 24, 28, rl.NewColor(179, 210, 145, 255))
		rl.DrawText(fmt.Sprintf("pig / %s", mode), 28, 65, 20, rl.White)
		rl.DrawText(status, 28, 710, 16, rl.LightGray)
		rl.EndDrawing()
	}
}
