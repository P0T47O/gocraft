package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) generateAtlas() {
	// Identify all unique block textures needed
	paths := []string{
		"textures/block/stone.png",
		"textures/block/dirt.png",
		"textures/block/grass_block_top.png",
		"textures/block/grass_block_side.png",
		"textures/block/cobblestone.png",
		"textures/block/oak_planks.png",
		"textures/block/bedrock.png",
		"textures/block/sand.png",
		"textures/block/gravel.png",
		"textures/block/gold_ore.png",
		"textures/block/iron_ore.png",
		"textures/block/coal_ore.png",
		"textures/block/oak_log.png",
		"textures/block/oak_log_top.png",
		"textures/block/oak_leaves.png", // cutout
		"textures/block/glass.png",      // glass
		"textures/block/diamond_ore.png",
		"textures/block/lapis_ore.png",
		"textures/block/torch.png", // cutout
		"textures/block/crafting_table_top.png",
		"textures/block/crafting_table_side.png",
		"textures/block/crafting_table_front.png",
		"textures/block/furnace_front.png",
		"textures/block/furnace_side.png",
		"textures/block/furnace_top.png",
		"textures/block/tnt_side.png",
		"textures/block/tnt_top.png",
		"textures/block/tnt_bottom.png",
	}

	// Dynamic sizing
	count := len(paths)
	cols := 0
	rows := 0
	size := atlasCellSize // Aligned tiles plus extruded edges for mip isolation.

	// Simple sqrt approximation for grid
	side := 1
	for side*side < count {
		side++
	}
	cols = side
	rows = side

	atlasWidth := int32(cols * size)
	atlasHeight := int32(rows * size)

	atlasImg := rl.GenImageColor(int(atlasWidth), int(atlasHeight), rl.Blank)
	uvs := map[string]rl.Rectangle{}

	for i, path := range paths {
		var img *rl.Image

		// Special handling for torch (resize canvas/center)
		if path == "textures/block/torch.png" {
			raw := rl.LoadImage(path)
			if raw != nil {
				img = rl.GenImageColor(16, 16, rl.Blank)
				src := rl.NewRectangle(0, 0, float32(raw.Width), float32(raw.Height))
				dst := rl.NewRectangle(7, 0, float32(raw.Width), float32(raw.Height)) // Center torch
				rl.ImageDraw(img, raw, src, dst, rl.White)
				rl.UnloadImage(raw)
			}
		} else if path == "textures/block/oak_leaves.png" || path == "textures/block/grass_block_side_overlay.png" {
			img = rl.LoadImage(path)
			// Apply alpha fix similar to loadTexture
			if img != nil && img.Data != nil {
				colors := rl.LoadImageColors(img)
				hasAlpha := false
				for _, c := range colors {
					if c.A < 255 {
						hasAlpha = true
						break
					}
				}
				if !hasAlpha {
					key := pickEdgeColorKey(colors, int(img.Width), int(img.Height))
					rl.ImageAlphaClear(img, key, 0.05)
				}
				rl.UnloadImageColors(colors)
			}
		} else if path == "textures/block/glass.png" {
			img = rl.LoadImage(path)
			// Apply glass alpha fix
			if img != nil && img.Data != nil {
				colors := rl.LoadImageColors(img)
				if len(colors) > 0 {
					w := int(img.Width)
					h := int(img.Height)
					for y := 0; y < h; y++ {
						for x := 0; x < w; x++ {
							idx := y*w + x
							c := colors[idx]
							if c.A > 0 && c.A < 255 {
								c.A = 255
								rl.ImageDrawPixel(img, int32(x), int32(y), c)
							}
						}
					}
					rl.UnloadImageColors(colors)
				}
			}
		} else {
			img = rl.LoadImage(path)
		}

		if img == nil || img.Data == nil {
			if img != nil {
				rl.UnloadImage(img)
			}
			continue
		}

		if img.Width != 16 || img.Height != 16 {
			rl.ImageCrop(img, rl.NewRectangle(0, 0, 16, 16))
		}

		// Calculate pos
		col := i % cols
		row := i / cols
		x := int32(col*size + atlasPadding)
		y := int32(row*size + atlasPadding)

		// Draw to atlas
		colors := rl.LoadImageColors(img)
		if len(colors) >= atlasTileSize*atlasTileSize {
			for py := 0; py < size; py++ {
				for px := 0; px < size; px++ {
					color := colors[atlasSourceCoordinate(py)*atlasTileSize+atlasSourceCoordinate(px)]
					rl.ImageDrawPixel(atlasImg, int32(col*size+px), int32(row*size+py), color)
				}
			}
		}
		rl.UnloadImageColors(colors)

		// Calculate UV (normalized 0-1)
		uvX := float32(x) / float32(atlasWidth)
		uvY := float32(y) / float32(atlasHeight)
		uvW := float32(img.Width) / float32(atlasWidth)
		uvH := float32(img.Height) / float32(atlasHeight)

		uvs[path] = rl.NewRectangle(uvX, uvY, uvW, uvH)

		rl.UnloadImage(img)
	}

	tex := rl.LoadTextureFromImage(atlasImg)
	configureWorldTexture(&tex, true)
	rl.UnloadImage(atlasImg)

	a.atlas = &TextureAtlas{
		Texture: tex,
		UVs:     uvs,
	}
}

func (a *RenderAssets) getAtlasUV(path string) (rl.Rectangle, bool) {
	if a.atlas == nil {
		return rl.NewRectangle(0, 0, 1, 1), false
	}
	uv, ok := a.atlas.UVs[path]
	return uv, ok
}
