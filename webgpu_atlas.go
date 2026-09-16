package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"sort"
)

type webGPUAtlasAnimation struct {
	frames         [][]byte
	width, height  uint32
	bytesPerRow    uint32
	atlasX, atlasY uint32
	frameSeconds   float32
	index          int
}

type webGPUBlockAtlas struct {
	pixels        []byte
	width, height int
	uvs           map[string]AtlasRect
	animations    []webGPUAtlasAnimation
}

// The experimental renderer is a single active instance. Keep its CPU-side
// animation frames next to the atlas builder so the Windows presenter can issue
// small partial texture uploads without involving Raylib texture state.
var activeWebGPUAtlasAnimations []webGPUAtlasAnimation

const webGPUAtlasMaxTileExtent = 64

func webGPUAtlasSourceCoordinate(pixel, extent int) int {
	return max(0, min(extent-1, pixel-atlasPadding))
}

func webGPUAtlasTile(path string, img image.Image) (*image.NRGBA, int, int, [][]byte, uint32, float32) {
	bounds := img.Bounds()

	// Animated block textures are vertical frame strips. Keep every frame in CPU
	// memory so the live WebGPU renderer can update only this atlas tile, while
	// frame zero is packed into the initial full-atlas upload.
	if isAnimatedTexture(path) {
		frameSize := bounds.Dx()
		if bounds.Dy() < frameSize {
			frameSize = bounds.Dy()
		}
		frameSize = max(frameSize, 1)
		frameCount := bounds.Dy() / frameSize
		if frameCount <= 0 {
			frameCount = 1
		}
		tileW := min(webGPUAtlasMaxTileExtent, frameSize)
		tileH := tileW
		rowBytes := tileW * 4
		// Keep rows 256-byte aligned so partial Queue.WriteTexture uploads are
		// accepted consistently by every backend.
		bytesPerRow := uint32((rowBytes + 255) &^ 255)
		frames := make([][]byte, 0, frameCount)
		var first *image.NRGBA
		for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
			tile := image.NewNRGBA(image.Rect(0, 0, tileW, tileH))
			frameY := bounds.Min.Y + frameIndex*frameSize
			for y := 0; y < tileH; y++ {
				sy := frameY + y*frameSize/tileH
				for x := 0; x < tileW; x++ {
					sx := bounds.Min.X + x*frameSize/tileW
					tile.Set(x, y, img.At(sx, sy))
				}
			}
			framePixels := make([]byte, int(bytesPerRow)*tileH)
			for y := 0; y < tileH; y++ {
				src := tile.Pix[y*tile.Stride : y*tile.Stride+rowBytes]
				dst := framePixels[y*int(bytesPerRow) : y*int(bytesPerRow)+rowBytes]
				copy(dst, src)
			}
			frames = append(frames, framePixels)
			if first == nil {
				first = tile
			}
		}

		frameSeconds := float32(0.1)
		if data, err := os.ReadFile(path + ".mcmeta"); err == nil {
			var meta struct {
				Animation struct {
					FrameTime int `json:"frametime"`
				} `json:"animation"`
			}
			if json.Unmarshal(data, &meta) == nil && meta.Animation.FrameTime > 0 {
				frameSeconds = float32(meta.Animation.FrameTime) / 20.0
			}
		}
		return first, tileW, tileH, frames, bytesPerRow, frameSeconds
	}

	if path == "textures/block/torch.png" && bounds.Dx() < atlasTileSize {
		tile := image.NewNRGBA(image.Rect(0, 0, atlasTileSize, atlasTileSize))
		dx := (atlasTileSize - min(atlasTileSize, bounds.Dx())) / 2
		dst := image.Rect(dx, 0, dx+min(atlasTileSize, bounds.Dx()), min(atlasTileSize, bounds.Dy()))
		draw.Draw(tile, dst, img, bounds.Min, draw.Src)
		return tile, atlasTileSize, atlasTileSize, nil, 0, 0
	}

	tileW := min(webGPUAtlasMaxTileExtent, bounds.Dx())
	tileH := min(webGPUAtlasMaxTileExtent, bounds.Dy())
	tileW = max(tileW, 1)
	tileH = max(tileH, 1)
	tile := image.NewNRGBA(image.Rect(0, 0, tileW, tileH))
	if bounds.Dx() == tileW && bounds.Dy() == tileH {
		draw.Draw(tile, tile.Bounds(), img, bounds.Min, draw.Src)
		return tile, tileW, tileH, nil, 0, 0
	}

	// Oversized sources are reduced with nearest-neighbor sampling. Mob skins are
	// currently 64x64, so they stay lossless instead of collapsing to a 16x16 tile.
	for y := 0; y < tileH; y++ {
		sy := bounds.Min.Y + y*bounds.Dy()/tileH
		for x := 0; x < tileW; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/tileW
			tile.Set(x, y, img.At(sx, sy))
		}
	}
	return tile, tileW, tileH, nil, 0, 0
}

// buildWebGPUBlockAtlas builds a CPU-side RGBA atlas containing every texture
// referenced by the current block/item registries plus mob skins and mining
// crack stages. The returned UV map is installed on RenderAssets so all native
// WebGPU gameplay presentation can share one texture without Raylib GPU state.
func buildWebGPUBlockAtlas() (*webGPUBlockAtlas, error) {
	pathsSet := make(map[string]struct{})
	for _, def := range Blocks {
		if def == nil {
			continue
		}
		faces := def.Textures
		for _, path := range []string{faces.Top, faces.Bottom, faces.North, faces.South, faces.East, faces.West} {
			if path != "" {
				pathsSet[path] = struct{}{}
			}
		}
	}

	// Standalone tools/items do not have block faces but still need to appear in
	// the WebGPU hotbar/inventory. Block items already reuse their block texture.
	for _, def := range Items {
		if def != nil && def.Icon != "" {
			pathsSet[def.Icon] = struct{}{}
		}
	}

	// Preserve native mob skin resolution inside the atlas cell. The existing
	// 256px cell with 112px left/top padding leaves room for current 64x64 skins.
	for _, model := range mobContent.Models {
		if model.Texture != "" {
			pathsSet[model.Texture] = struct{}{}
		}
	}

	for i := 0; i < 10; i++ {
		pathsSet[fmt.Sprintf("textures/block/destroy_stage_%d.png", i)] = struct{}{}
	}

	// Grass side overlay is emitted directly by the chunk mesher rather than
	// being referenced by BlockDef.Textures.
	pathsSet["textures/block/grass_block_side_overlay.png"] = struct{}{}

	paths := make([]string, 0, len(pathsSet))
	for path := range pathsSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("registry contains no texture paths")
	}

	side := 1
	for side*side < len(paths) {
		side++
	}
	width := side * atlasCellSize
	height := side * atlasCellSize
	pixels := make([]byte, width*height*4)
	uvs := make(map[string]AtlasRect, len(paths))
	animations := make([]webGPUAtlasAnimation, 0, 4)

	for i, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		img, _, decodeErr := image.Decode(file)
		_ = file.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode %s: %w", path, decodeErr)
		}

		tile, tileW, tileH, animationFrames, animationBytesPerRow, frameSeconds := webGPUAtlasTile(path, img)
		col := i % side
		row := i / side
		for py := 0; py < atlasCellSize; py++ {
			sy := webGPUAtlasSourceCoordinate(py, tileH)
			for px := 0; px < atlasCellSize; px++ {
				sx := webGPUAtlasSourceCoordinate(px, tileW)
				c := tile.NRGBAAt(sx, sy)
				dstX := col*atlasCellSize + px
				dstY := row*atlasCellSize + py
				offset := (dstY*width + dstX) * 4
				pixels[offset+0] = c.R
				pixels[offset+1] = c.G
				pixels[offset+2] = c.B
				pixels[offset+3] = c.A
			}
		}

		uvs[path] = AtlasRect{
			X:      float32(col*atlasCellSize+atlasPadding) / float32(width),
			Y:      float32(row*atlasCellSize+atlasPadding) / float32(height),
			Width:  float32(tileW) / float32(width),
			Height: float32(tileH) / float32(height),
		}
		if len(animationFrames) > 1 && frameSeconds > 0 {
			animations = append(animations, webGPUAtlasAnimation{
				frames:        animationFrames,
				width:         uint32(tileW),
				height:        uint32(tileH),
				bytesPerRow:   animationBytesPerRow,
				atlasX:        uint32(col*atlasCellSize + atlasPadding),
				atlasY:        uint32(row*atlasCellSize + atlasPadding),
				frameSeconds:  frameSeconds,
				index:         0,
			})
		}
	}

	activeWebGPUAtlasAnimations = animations
	return &webGPUBlockAtlas{pixels: pixels, width: width, height: height, uvs: uvs, animations: animations}, nil
}
