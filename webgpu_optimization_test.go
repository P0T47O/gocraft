package main

import (
	"math"
	"testing"
)

func TestWebGPUMipsAlphaAndDimensions(t *testing.T) {
	mips := buildWebGPUMips([]byte{255, 0, 0, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 2, 2, 5)
	if len(mips) != 2 || mips[1].width != 1 || mips[1].height != 1 {
		t.Fatal("invalid mip dimensions")
	}
	got := mips[1].pixels
	if got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 64 {
		t.Fatalf("alpha fringe: %v", got)
	}
}

func TestWebGPUAnimatedMipPadding(t *testing.T) {
	a := webGPUAtlasAnimation{width: 1, height: 1, bytesPerRow: 4, frames: [][]byte{{255, 0, 0, 255}, {0, 255, 0, 255}}}
	for index := 0; index < 2; index++ {
		for level, mip := range a.mipFrame(index) {
			if mip.width != atlasCellSize>>level {
				t.Fatal("wrong size")
			}
			for i := 0; i < len(mip.pixels); i += 4 {
				for c := 0; c < 4; c++ {
					if mip.pixels[i+c] != a.frames[index][c] {
						t.Fatalf("stale padding/mip %d frame %d", level, index)
					}
				}
			}
		}
	}
}

func TestMeshMathMatchesReference(t *testing.T) {
	for _, axis := range []gameVec3{{0, 1, 0}, {1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {1, 2, 3}} {
		for _, angle := range []float32{0, .3, -.7, 1.57} {
			v := meshVec3(.2, .5, -.1)
			got := meshTransform(v, meshRotation(axis, angle))
			// Rodrigues' formula in float64 is independent of the matrix library.
			x, y, z := float64(axis.X), float64(axis.Y), float64(axis.Z)
			length := math.Sqrt(x*x + y*y + z*z)
			x, y, z = x/length, y/length, z/length
			vx, vy, vz := float64(v.X), float64(v.Y), float64(v.Z)
			c, s := math.Cos(float64(angle)), math.Sin(float64(angle))
			dot := x*vx + y*vy + z*vz
			want := gameVec3{
				float32(vx*c + (y*vz-z*vy)*s + x*dot*(1-c)),
				float32(vy*c + (z*vx-x*vz)*s + y*dot*(1-c)),
				float32(vz*c + (x*vy-y*vx)*s + z*dot*(1-c)),
			}
			if math.Abs(float64(got.X-want.X))+math.Abs(float64(got.Y-want.Y))+math.Abs(float64(got.Z-want.Z)) > 1e-5 {
				t.Fatalf("transform changed: %v vs %v", got, want)
			}
		}
	}
}

func TestWebGPUCPUAssetsHaveNoLegacyResources(t *testing.T) {
	a := newWebGPUCPUAssets()
	if a.atlas == nil {
		t.Fatal("missing CPU atlas metadata")
	}
	a.unload() // must be safe with no window or graphics context
}

func TestSolidMeshWinding(t *testing.T) {
	initBlockRegistry()
	a := newWebGPUCPUAssets()
	var heights [chunkWidth][chunkWidth]int16
	data := a.buildAllMeshData(&heights, 0, 0, 0, 16,
		func(x, y, z int) byte {
			if x == 8 && y == 8 && z == 8 {
				return blockStone
			}
			return blockAir
		},
		func(x, y, z int) byte { return 15 }, func(x, y, z int) byte { return 0 }, 123)
	defer releaseMeshResults(data)
	triangles := 0
	for _, list := range data["opaque"] {
		for _, d := range list {
			for i := 0; i < len(d.indices); i += 3 {
				p, q, r := int(d.indices[i])*3, int(d.indices[i+1])*3, int(d.indices[i+2])*3
				u := meshVec3(d.vertices[q]-d.vertices[p], d.vertices[q+1]-d.vertices[p+1], d.vertices[q+2]-d.vertices[p+2])
				v := meshVec3(d.vertices[r]-d.vertices[p], d.vertices[r+1]-d.vertices[p+1], d.vertices[r+2]-d.vertices[p+2])
				n := meshCross(u, v)
				if n.X*d.normals[p]+n.Y*d.normals[p+1]+n.Z*d.normals[p+2] <= 0 {
					t.Fatalf("inward triangle %d normal %v", i, d.normals[p:p+3])
				}
				triangles++
			}
		}
	}
	if triangles != 12 {
		t.Fatalf("expected six faces, got %d triangles", triangles)
	}
}
