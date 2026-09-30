package main

import (
	"image"
	"image/color"
	"sync/atomic"
)

type lodMaterialPalette [256][4]uint8

// Publish an immutable palette before LOD workers start. Atomic publication
// also keeps renderer recreation safe while old readers finish a tile.
var activeLODMaterialPalette atomic.Pointer[lodMaterialPalette]

func lodTextureAverage(img image.Image) [4]uint8 {
	var sums [3]uint64
	var weight uint64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			a := uint64(c.A)
			sums[0] += uint64(c.R) * a
			sums[1] += uint64(c.G) * a
			sums[2] += uint64(c.B) * a
			weight += a
		}
	}
	if weight == 0 {
		return [4]uint8{}
	}
	return [4]uint8{uint8((sums[0] + weight/2) / weight), uint8((sums[1] + weight/2) / weight), uint8((sums[2] + weight/2) / weight), 255}
}

func publishLODMaterialPalette(averages map[string][4]uint8) {
	palette := new(lodMaterialPalette)
	for _, block := range Blocks {
		if block != nil {
			palette[block.ID] = averages[block.Textures.Top]
		}
	}
	activeLODMaterialPalette.Store(palette)
}
