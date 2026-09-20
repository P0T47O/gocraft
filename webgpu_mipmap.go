package main

// Mip pixels use straight alpha. Weight RGB by coverage to avoid dark fringes
// from transparent black texels; source atlas cells remain aligned at each LOD.
type webGPUMip struct {
	width, height int
	pixels        []byte
}

func buildWebGPUMips(pixels []byte, width, height, levels int) []webGPUMip {
	mips := []webGPUMip{{width, height, pixels}}
	for len(mips) < levels && (width > 1 || height > 1) {
		width, height = max(1, width/2), max(1, height/2)
		mips = append(mips, webGPUMip{width, height, make([]byte, width*height*4)})
	}
	refreshWebGPUMips(mips)
	return mips
}

func refreshWebGPUMips(mips []webGPUMip) {
	for level := 1; level < len(mips); level++ {
		src, dst := mips[level-1], mips[level]
		for y := 0; y < dst.height; y++ {
			for x := 0; x < dst.width; x++ {
				var sums [4]int
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						i := (min(src.height-1, y*2+dy)*src.width + min(src.width-1, x*2+dx)) * 4
						a := int(src.pixels[i+3])
						for c := 0; c < 3; c++ {
							sums[c] += int(src.pixels[i+c]) * a
						}
						sums[3] += a
					}
				}
				i := (y*dst.width + x) * 4
				for c := 0; c < 3; c++ {
					dst.pixels[i+c] = 0
					if sums[3] > 0 {
						dst.pixels[i+c] = byte((sums[c] + sums[3]/2) / sums[3])
					}
				}
				dst.pixels[i+3] = byte((sums[3] + 2) / 4)
			}
		}
	}
}

// Refresh the complete extruded cell, not just its interior: linear/AF taps
// must never sample the previous animation frame in the padding.
func (a *webGPUAtlasAnimation) mipFrame(index int) []webGPUMip {
	if len(a.mips) == 0 {
		a.mips = buildWebGPUMips(make([]byte, atlasCellSize*atlasCellSize*4), atlasCellSize, atlasCellSize, atlasMaxMipLevel+1)
	}
	frame := a.frames[index]
	for y := 0; y < atlasCellSize; y++ {
		for x := 0; x < atlasCellSize; x++ {
			sx := webGPUAtlasSourceCoordinate(x, int(a.width))
			sy := webGPUAtlasSourceCoordinate(y, int(a.height))
			src := sy*int(a.bytesPerRow) + sx*4
			dst := (y*atlasCellSize + x) * 4
			copy(a.mips[0].pixels[dst:dst+4], frame[src:src+4])
		}
	}
	refreshWebGPUMips(a.mips)
	return a.mips
}
