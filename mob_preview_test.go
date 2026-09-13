package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"runtime"
	"testing"
)

func TestMobRenderPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_MOB_PREVIEW") != "1" {
		t.Skip("opt-in graphical check")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1000, 700, "Mob preview test")
	defer rl.CloseWindow()
	r, err := newMobRenderer(mobContent)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	os.MkdirAll("work", 0755)
	cam := rl.Camera3D{Position: rl.NewVector3(2.3, 1.7, 3), Target: rl.NewVector3(0, .5, 0), Up: rl.NewVector3(0, 1, 0), Fovy: 45}
	for _, state := range []string{"idle", "walk", "hurt", "death"} {
		e := &RemoteEntity{MobKind: "pig", MobHealth: 10}
		if state == "walk" {
			e.AnimBlend = 1
			e.AnimPhase = 1
		}
		if state == "hurt" {
			e.MobHurt = 1
		}
		if state == "death" {
			e.DeathTime = .6
		}
		target := rl.LoadRenderTexture(1000, 700)
		rl.BeginTextureMode(target)
		rl.ClearBackground(rl.NewColor(22, 32, 35, 255))
		rl.BeginMode3D(cam)
		rl.DrawGrid(8, 1)
		r.Draw(e, true)
		rl.EndMode3D()
		rl.DrawText("PIG / "+state, 25, 25, 25, rl.White)
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(target.Texture)
		rl.ImageFlipVertical(img)
		ok := rl.ExportImage(*img, "work/pig-"+state+".png")
		rl.UnloadImage(img)
		rl.UnloadRenderTexture(target)
		if !ok {
			t.Fatal("image export")
		}
	}
}
