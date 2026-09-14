package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) initIcons() {
	for _, block := range allBlocks {
		a.makeIcon(block)
	}
}

func (a *RenderAssets) makeIcon(block byte) rl.RenderTexture2D {
	if rt, ok := a.iconRenders[block]; ok {
		return rt
	}
	rt := rl.LoadRenderTexture(64, 64)
	if rt.ID != 0 {
		rl.BeginTextureMode(rt)
		rl.ClearBackground(rl.Blank)
		rl.EnableDepthTest()
		def := GetItemVisual(block)
		if def.RenderType == RenderTypeCross {
			// Render flat 2D sprite for cross type
			texPath := def.Textures.North
			tex := a.textures[texPath]
			if tex.ID != 0 {
				// Draw texture scaled to fit 64x64
				// Keep aspect ratio if needed, but usually 1:1 for icons
				scale := float32(64) / float32(tex.Width)
				if float32(tex.Height)*scale > 64 {
					scale = 64 / float32(tex.Height)
				}
				destW := float32(tex.Width) * scale
				destH := float32(tex.Height) * scale
				destX := (64 - destW) / 2
				destY := (64 - destH) / 2

				tint := rl.White
				if block == blockTallGrass {
					tint = rl.NewColor(145, 189, 89, 255)
				}
				rl.DrawTextureEx(tex, rl.NewVector2(destX, destY), 0, scale, tint)
			}
		} else {
			rl.BeginMode3D(a.iconCamera)

			// Shading for icon: Top is bright, Front/Side have different shades
			getShadedLight := func(ix, iy, iz int) byte {
				if iy > 0 {
					return 15 // Top
				}
				if iz < 0 {
					return 12 // North
				}
				if iz > 0 {
					return 14 // South
				}
				if ix < 0 {
					return 10 // West
				}
				if ix > 0 {
					return 11 // East
				}
				return 13
			}

			if block == blockLeaves {
				rl.DisableBackfaceCulling()
			}
			a.drawBlock(block, rl.NewVector3(0, 0, 0), func(nx, ny, nz int) byte {
				return blockAir
			}, getShadedLight, func(ix, iy, iz int) byte {
				return 0
			}, 0, 0, 0)
			if block == blockLeaves {
				rl.EnableBackfaceCulling()
			}

			rl.EndMode3D()
		}
		rl.DisableDepthTest()
		rl.EndTextureMode()
		rl.SetTextureFilter(rt.Texture, rl.FilterPoint)
		a.iconRenders[block] = rt
		a.iconOffsets[block] = measureIconOffset(rt.Texture)
	}
	return rt
}

func measureIconOffset(tex rl.Texture2D) rl.Vector2 {
	img := rl.LoadImageFromTexture(tex)
	if img == nil || img.Data == nil {
		if img != nil {
			rl.UnloadImage(img)
		}
		return rl.NewVector2(0, 0)
	}
	colors := rl.LoadImageColors(img)
	w := int(img.Width)
	h := int(img.Height)
	minX := w
	minY := h
	maxX := -1
	maxY := -1
	if len(colors) >= w*h {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := colors[y*w+x]
				if c.A > 5 {
					if x < minX {
						minX = x
					}
					if y < minY {
						minY = y
					}
					if x > maxX {
						maxX = x
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	}
	rl.UnloadImageColors(colors)
	rl.UnloadImage(img)
	if maxX < 0 || maxY < 0 {
		return rl.NewVector2(0, 0)
	}
	centerX := float32(minX+maxX+1) * 0.5
	centerY := float32(minY+maxY+1) * 0.5
	texCenterX := float32(w) * 0.5
	texCenterY := float32(h) * 0.5
	return rl.NewVector2(centerX-texCenterX, centerY-texCenterY)
}
