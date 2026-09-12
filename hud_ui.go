package main

import rl "github.com/gen2brain/raylib-go/raylib"

func drawChatOverlay() {
	if len(chatHistory) == 0 && !isChatOpen {
		return
	}
	scale := min(float32(1.4), float32(rl.GetScreenHeight())/720)
	width := min(float32(520)*scale, float32(rl.GetScreenWidth())-32)
	lineHeight := 22 * scale
	count := min(6, len(chatHistory))
	bottom := float32(rl.GetScreenHeight()) - 106*scale
	top := bottom - float32(count)*lineHeight - 16*scale
	rect := rl.NewRectangle(16, top, width, float32(count)*lineHeight+16*scale)
	if count > 0 {
		inventoryBox(rect, rl.Fade(invBackground, 0.93), invLine)
		rl.BeginScissorMode(int32(rect.X+8), int32(rect.Y), int32(rect.Width-16), int32(rect.Height))
		for i, msg := range chatHistory[len(chatHistory)-count:] {
			inventoryText(msg, 24, top+8*scale+float32(i)*lineHeight, int32(15*scale), invText)
		}
		rl.EndScissorMode()
	}
	if isChatOpen {
		inputRect := rl.NewRectangle(16, bottom+4*scale, width, 32*scale)
		inventoryBox(inputRect, invBackground, invAccent)
		text := "> " + chatInput + "_"
		fs := int32(16 * scale)
		for len(text) > 0 && float32(rl.MeasureText(text, fs)) > width-20 {
			text = string([]rune(text)[1:])
		}
		inventoryText(text, 26, inputRect.Y+8*scale, fs, invText)
	}
}
