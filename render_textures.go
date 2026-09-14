package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
)

func (a *RenderAssets) loadTexture(path string) rl.Texture2D {
	a.mu.RLock()
	if tex, ok := a.textures[path]; ok {
		a.mu.RUnlock()
		return tex
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	// Double check
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	var tex rl.Texture2D
	if isAnimatedTexture(path) {
		tex = a.loadAnimatedTexture(path)
	} else if path == "textures/block/torch.png" {
		img := rl.LoadImage(path)
		if img != nil && img.Data != nil {
			if img.Width == 2 && img.Height == 16 {
				canvas := rl.GenImageColor(16, 16, rl.Blank)
				src := rl.NewRectangle(0, 0, float32(img.Width), float32(img.Height))
				dst := rl.NewRectangle(7, 0, float32(img.Width), float32(img.Height))
				rl.ImageDraw(canvas, img, src, dst, rl.White)
				rl.UnloadImage(img)
				img = canvas
			}
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	} else if path == "textures/block/oak_leaves.png" || path == "textures/block/grass_block_side_overlay.png" {
		img := rl.LoadImage(path)
		if img != nil && img.Data != nil {
			colors := rl.LoadImageColors(img)
			hasAlpha := false
			if len(colors) > 0 {
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
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	} else if path == "textures/block/glass.png" {
		img := rl.LoadImage(path)
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
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	}
	if tex.ID == 0 {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File missing: Generate Magenta/Black Checkerboard
			img := rl.GenImageChecked(16, 16, 8, 8, rl.Magenta, rl.Black)
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		} else {
			// File exists: Load it
			img := rl.LoadImage(path)
			if img != nil && img.Data != nil {
				w := float32(img.Width)
				h := float32(img.Height)
				if h > w {
					rl.ImageCrop(img, rl.NewRectangle(0, 0, w, w))
				}
				tex = rl.LoadTextureFromImage(img)
				rl.UnloadImage(img)
			} else {
				tex = rl.LoadTexture(path)
			}
		}
	}

	if tex.ID != 0 {
		rl.SetTextureFilter(tex, rl.FilterPoint)
	}
	a.textures[path] = tex
	return tex
}

func (a *RenderAssets) getMaterial(path string, tex rl.Texture2D) rl.Material {
	a.mu.RLock()
	if mat, ok := a.materials[path]; ok {
		a.mu.RUnlock()
		return mat
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	// Double check
	if mat, ok := a.materials[path]; ok {
		return mat
	}

	material := rl.LoadMaterialDefault()
	if material.Maps == nil {
		fmt.Printf("ERROR: LoadMaterialDefault returned nil Maps for %s\n", path)
	}
	if tex.ID != 0 {
		// fmt.Printf("DEBUG: SetMaterialTexture [%s] MatPtr=%p Maps=%p TexID=%d\n", path, &material, material.Maps, tex.ID)
		rl.SetMaterialTexture(&material, rl.MapDiffuse, tex)
	}

	if a.fogShader.ID != 0 {
		material.Shader = a.fogShader
	} else if (path == "textures/block/oak_leaves.png" || path == "textures/block/torch.png") && a.cutoutShader.ID != 0 {
		material.Shader = a.cutoutShader
	}

	a.materials[path] = material
	return material
}

func pickEdgeColorKey(colors []rl.Color, w, h int) rl.Color {
	if w <= 0 || h <= 0 || len(colors) < w*h {
		return rl.White
	}
	counts := map[uint32]int{}
	encode := func(c rl.Color) uint32 {
		return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
	}
	track := func(x, y int) {
		c := colors[y*w+x]
		counts[encode(c)]++
	}
	for x := 0; x < w; x++ {
		track(x, 0)
		track(x, h-1)
	}
	for y := 1; y < h-1; y++ {
		track(0, y)
		track(w-1, y)
	}
	var best rl.Color
	bestCount := -1
	for k, v := range counts {
		if v > bestCount {
			bestCount = v
			best = rl.NewColor(uint8(k>>24), uint8(k>>16), uint8(k>>8), uint8(k))
		}
	}
	if bestCount < 0 {
		return rl.White
	}
	return best
}
