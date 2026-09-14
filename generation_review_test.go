package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerationWeightsAndSurfaceTransitions(t *testing.T) {
	transitionGrass, transitionSand, rock, highGrass := 0, 0, 0, 0
	for x := -3072; x <= 3072; x += 13 {
		for z := -3072; z <= 3072; z += 13 {
			c := sampleTerrainColumn(42, x, z)
			sum := float32(0)
			for _, w := range c.environment.weights {
				if w < 0 || w > 1 {
					t.Fatal("invalid weight")
				}
				sum += w
			}
			if abs(sum-1) > .00001 {
				t.Fatal("weights not normalized")
			}
			d := c.environment.weights[regionDesert]
			if d > .35 && d < .65 && c.height > seaLevel+6 {
				if c.top == blockGrass {
					transitionGrass++
				}
				if c.top == blockSand {
					transitionSand++
				}
			}
			if c.environment.weights[regionMountain] > .5 {
				if c.top == blockStone {
					rock++
				}
				if c.height > 94 && c.top == blockGrass {
					highGrass++
				}
			}
		}
	}
	if transitionGrass < 50 || transitionSand < 50 || rock < 20 || highGrass < 50 {
		t.Fatalf("missing transitions/rock/upper grass: %d %d %d %d", transitionGrass, transitionSand, rock, highGrass)
	}
	t.Logf("transition grass/sand=%d/%d, rock=%d, upper grass=%d", transitionGrass, transitionSand, rock, highGrass)
}

func TestGenerationRockUsesSlopeNotAltitude(t *testing.T) {
	e := environmentSample{height: 90, elevation: 90}
	e.weights[regionMountain] = 1
	for _, height := range []int{90, 94, 95, 110, 130} {
		e.height = height
		e.elevation = float32(height)
		gentle, _ := surfaceFromEnvironment(42, 10, 20, e, .1)
		steep, _ := surfaceFromEnvironment(42, 10, 20, e, 1.1)
		if gentle != blockGrass || steep != blockStone {
			t.Fatalf("altitude %d: gentle %d steep %d", height, gentle, steep)
		}
	}
}

// Opt-in, reproducible CPU diagnostic, not a screenshot of the game. Each pixel
// samples the real generator; light direction is fixed for comparison.
func TestGenerationReviewMaps(t *testing.T) {
	dir := os.Getenv("GOCRAFT_REVIEW_DIR")
	if dir == "" {
		t.Skip("set GOCRAFT_REVIEW_DIR to export terrain diagnostics")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	seed := uint32(42)
	// Pick a strong inland mountain, then include 1536 blocks around it.
	centerX, centerZ := 0, 0
	best := float32(-1)
	for x := -2048; x <= 2048; x += 64 {
		for z := -2048; z <= 2048; z += 64 {
			e := sampleEnvironment(seed, x, z)
			score := e.weights[regionMountain] * e.land
			if score > best {
				best = score
				centerX = x
				centerZ = z
			}
		}
	}
	t.Logf("review seed=%d center=%d,%d extent=1536", seed, centerX, centerZ)
	for _, kind := range []string{"surface", "height", "slope"} {
		img := image.NewRGBA(image.Rect(0, 0, 512, 512))
		for px := 0; px < 512; px++ {
			for pz := 0; pz < 512; pz++ {
				x, z := centerX+(px-256)*3, centerZ+(pz-256)*3
				c := sampleTerrainColumn(seed, x, z)
				r, g, b := float32(94), float32(142), float32(67)
				switch c.top {
				case blockSand:
					r, g, b = 218, 203, 148
				case blockStone:
					r, g, b = 133, 135, 132
				case blockSnow:
					r, g, b = 237, 241, 244
				case blockGravel:
					r, g, b = 126, 123, 115
				case blockDirt:
					r, g, b = 134, 105, 72
				}
				if c.height < seaLevel {
					r, g, b = 55, 115, 174
				}
				dx := (sampleEnvironment(seed, x+2, z).elevation - sampleEnvironment(seed, x-2, z).elevation) / 4
				dz := (sampleEnvironment(seed, x, z+2).elevation - sampleEnvironment(seed, x, z-2).elevation) / 4
				shade := max(float32(.35), min(float32(1.2), (.85-dx*.45-dz*.3)/float32(math.Sqrt(float64(1+dx*dx+dz*dz)))))
				if kind == "height" {
					r = c.environment.elevation * 1.8
					g = r
					b = r
				}
				if kind == "slope" {
					r = min(c.slope*220, 255)
					g = 60
					b = 30
					shade = 1
				}
				img.SetRGBA(px, pz, color.RGBA{uint8(min(r*shade, 255)), uint8(min(g*shade, 255)), uint8(min(b*shade, 255)), 255})
			}
		}
		f, err := os.Create(filepath.Join(dir, kind+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, img)
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}
