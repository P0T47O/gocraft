//go:build windows

package main

import (
	"math"

	"github.com/gogpu/wgpu"
)

func webGPUSelectedItem(state *InputState) byte {
	if state == nil || state.SelectedSlot < 0 || state.SelectedSlot >= 9 {
		return 0
	}
	if currentGameMode == ModeCreative || client == nil {
		return state.Hotbar[state.SelectedSlot]
	}
	stack := client.Inventory.Slots[state.SelectedSlot]
	if stack.ID <= 0 || stack.ID > 255 {
		return 0
	}
	return byte(stack.ID)
}

func (b *webGPUHUDBuilder) texturedQuad(p0, p1, p2, p3 [2]float32, u0, v0, u1, v1 float32, color [4]float32) {
	q0, q1, q2, q3 := b.point(p0[0], p0[1]), b.point(p1[0], p1[1]), b.point(p2[0], p2[1]), b.point(p3[0], p3[1])
	b.vertices = append(b.vertices,
		webGPUHUDVertex{Position: q0, UV: [2]float32{u0, v0}, Color: color},
		webGPUHUDVertex{Position: q1, UV: [2]float32{u1, v0}, Color: color},
		webGPUHUDVertex{Position: q2, UV: [2]float32{u1, v1}, Color: color},
		webGPUHUDVertex{Position: q0, UV: [2]float32{u0, v0}, Color: color},
		webGPUHUDVertex{Position: q2, UV: [2]float32{u1, v1}, Color: color},
		webGPUHUDVertex{Position: q3, UV: [2]float32{u0, v1}, Color: color},
	)
}

func drawWebGPUViewmodel(pass *wgpu.RenderPassEncoder, worldRenderer *webGPUWorldRenderer, state *InputState, now float32) error {
	if pass == nil || worldRenderer == nil || state == nil || state.InventoryOpen || isPaused || state.isDead() {
		return nil
	}
	id := webGPUSelectedItem(state)
	if id == 0 {
		return nil
	}
	u0, v0, u1, v1, ok := webGPUItemUV(id)
	if !ok {
		return nil
	}
	hud, err := ensureWebGPUHUDRenderer(worldRenderer)
	if err != nil {
		return err
	}

	b := newWebGPUHUDBuilder(worldRenderer.width, worldRenderer.height)
	scale := webGPUHUDScale(worldRenderer.width, worldRenderer.height)
	w := float32(170) * scale
	h := float32(170) * scale
	cx := float32(worldRenderer.width) - 118*scale
	cy := float32(worldRenderer.height) - 112*scale

	// Mining drives a compact first-person swing. Keeping this in screen space
	// avoids depth clipping against nearby blocks while the world renderer is
	// still using the same pass/depth attachment.
	swing := float32(0)
	if state.MiningProgress > 0 {
		swing = float32(math.Sin(float64(state.MiningProgress*math.Pi)))
	}
	idle := float32(math.Sin(float64(now*2.2))) * 2 * scale
	cx -= swing * 42 * scale
	cy += swing*50*scale + idle
	angle := float32(-0.30) + swing*0.55
	ca, sa := float32(math.Cos(float64(angle))), float32(math.Sin(float64(angle)))
	transform := func(x, y float32) [2]float32 {
		rx := x*ca - y*sa
		ry := x*sa + y*ca
		return [2]float32{cx + rx, cy + ry}
	}

	// Slight trapezoid gives the otherwise-flat item sprite a viewmodel-like
	// perspective. The atlas texture remains pixel-sharp and uses the same UVs as
	// inventory/hotbar presentation.
	p0 := transform(-w*0.42, -h*0.50)
	p1 := transform(w*0.50, -h*0.36)
	p2 := transform(w*0.42, h*0.50)
	p3 := transform(-w*0.50, h*0.36)
	shadowOffset := 5 * scale
	shadow := [4]float32{0, 0, 0, 0.34}
	b.texturedQuad(
		[2]float32{p0[0] + shadowOffset, p0[1] + shadowOffset},
		[2]float32{p1[0] + shadowOffset, p1[1] + shadowOffset},
		[2]float32{p2[0] + shadowOffset, p2[1] + shadowOffset},
		[2]float32{p3[0] + shadowOffset, p3[1] + shadowOffset},
		u0, v0, u1, v1, shadow,
	)
	b.texturedQuad(p0, p1, p2, p3, u0, v0, u1, v1, [4]float32{1, 1, 1, 1})
	return hud.drawPrepared(pass, b)
}
