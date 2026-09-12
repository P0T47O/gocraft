package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func (s *InputState) isDead() bool { return s.VitalsReady && s.Vitals.Health <= 0 }
func (s *InputState) updateRespawnRequest(c *Client) {
	if s.RespawnWaiting && c != nil && (s.RespawnLast == 0 || rl.GetTime()-s.RespawnLast > 1) {
		c.Send(&PacketRespawn{})
		s.RespawnLast = rl.GetTime()
	}
}
func drawVitalsHUD(s *InputState) {
	if !s.VitalsReady || currentGameMode != ModeSurvival {
		return
	}
	scale := min(float32(1.5), float32(rl.GetScreenWidth())/1280, float32(rl.GetScreenHeight())/720)
	x := float32(rl.GetScreenWidth())/2 - 240*scale
	y := float32(rl.GetScreenHeight()) - 126*scale
	inventoryBox(rl.NewRectangle(x-8*scale, y-7*scale, 242*scale, 29*scale), invBackground, invLine)
	mask := []string{"0110110", "1111111", "1111111", "0111110", "0011100", "0001000"}
	for i := 0; i < 10; i++ {
		for row, line := range mask {
			for col, c := range line {
				if c == '1' {
					color := invLine
					hp := int(s.Vitals.Health) - i*2
					if hp >= 2 || (hp == 1 && col < 3) {
						color = rl.NewColor(216, 111, 100, 255)
					}
					rl.DrawRectangleRec(rl.NewRectangle(x+float32(i)*18*scale+float32(col)*2*scale, y+float32(row)*2*scale, 2*scale, 2*scale), color)
				}
			}
		}
	}
	inventoryText(fmt.Sprintf("%d / 20", s.Vitals.Health), x+185*scale, y+2*scale, int32(12*scale), invText)
	if s.Vitals.Air < maxAir {
		bx := float32(rl.GetScreenWidth())/2 + 12*scale
		inventoryBox(rl.NewRectangle(bx, y-7*scale, 226*scale, 29*scale), invBackground, invLine)
		inventoryText("AIR", bx+8*scale, y+2*scale, int32(12*scale), invMuted)
		for i := 0; i < 10; i++ {
			color := invLine
			if s.Vitals.Air > int32(i*30) {
				color = rl.NewColor(138, 195, 213, 255)
			}
			rl.DrawCircleV(rl.NewVector2(bx+(48+float32(i)*16)*scale, y+6*scale), 5*scale, color)
		}
	}
	if s.HurtFlash > 0 {
		rl.DrawRectangleLinesEx(rl.NewRectangle(0, 0, float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())), 8*scale, rl.Fade(rl.NewColor(216, 80, 60, 255), s.HurtFlash*2))
	}
	if s.Vitals.Fire > 0 {
		inventoryText("BURNING - find water", x, y-24*scale, int32(13*scale), invWarning)
	} else if s.IsSwimming {
		inventoryText("SWIM  Space: up / Shift: down", x, y-24*scale, int32(12*scale), invMuted)
	} else if s.IsSneaking {
		inventoryText("SNEAKING", x, y-24*scale, int32(12*scale), invAccent)
	} else if s.IsRunning {
		inventoryText("SPRINTING", x, y-24*scale, int32(12*scale), invAccent)
	}
}
func drawDeathScreen(s *InputState) {
	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(28, 13, 16, 210))
	l := menuLayout()
	inventoryBox(l.Rect(220, 144, 560, 340), invBackground, invLine)
	rl.DrawRectangleRec(l.Rect(220, 144, 560, 3), invWarning)
	l.Text("YOU DIED", 252, 176, 32, invText)
	l.Text(s.Vitals.Cause, 252, 226, 18, invWarning)
	l.Text("Your items were dropped at the death location.", 252, 264, 14, invMuted)
	l.Text("Respawn restores health and air.", 252, 289, 14, invMuted)
	label := "Respawn"
	if s.RespawnWaiting {
		label = "Preparing a safe spawn..."
	}
	if ui.DrawAction(l.Rect(252, 338, 496, 42), label, !s.RespawnWaiting, true) {
		s.RespawnWaiting = true
		s.RespawnLast = 0
	}
	if ui.DrawButton(l.Rect(252, 394, 496, 38), "Return to menu", true) {
		exitGame()
	}
}
