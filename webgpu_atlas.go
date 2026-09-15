package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type webGPUBlockAtlas struct {
	pixels        []byte
	width, height int
	uvs           map[string]rl.Rectangle
}

// buildWebGPUBlockAtlas builds a CPU-side RGBA atlas containing every texture
// referenced by the current block registry plus standalone item sprites. The
// returned UV map is installed on RenderAssets so both chunk meshes and the
// native WebGPU UI can share one texture without depending on Raylib GPU state.
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
	uvs := make(map[string]rl.Rectangle, len(paths))

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

		tile := image.NewNRGBA(image.Rect(0, 0, atlasTileSize, atlasTileSize))
		bounds := img.Bounds()
		if path == "textures/block/torch.png" && bounds.Dx() < atlasTileSize {
			dx := (atlasTileSize - min(atlasTileSize, bounds.Dx())) / 2
			dst := image.Rect(dx, 0, dx+min(atlasTileSize, bounds.Dx()), min(atlasTileSize, bounds.Dy()))
			draw.Draw(tile, dst, img, bounds.Min, draw.Src)
		} else {
			dst := image.Rect(0, 0, min(atlasTileSize, bounds.Dx()), min(atlasTileSize, bounds.Dy()))
			draw.Draw(tile, dst, img, bounds.Min, draw.Src)
		}

		col := i % side
		row := i / side
		for py := 0; py < atlasCellSize; py++ {
			sy := atlasSourceCoordinate(py)
			for px := 0; px < atlasCellSize; px++ {
				sx := atlasSourceCoordinate(px)
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

		uvs[path] = rl.NewRectangle(
			float32(col*atlasCellSize+atlasPadding)/float32(width),
			float32(row*atlasCellSize+atlasPadding)/float32(height),
			float32(atlasTileSize)/float32(width),
			float32(atlasTileSize)/float32(height),
		)
	}

	return &webGPUBlockAtlas{pixels: pixels, width: width, height: height, uvs: uvs}, nil
}
