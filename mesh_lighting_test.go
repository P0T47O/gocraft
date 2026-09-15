package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"testing"
)

func TestSmoothLightSixFaces(t *testing.T) {
	initBlockRegistry()
	air := func(x, y, z int) byte { return blockAir }
	for face, f := range faceLightLayouts {
		light := func(x, y, z int) byte {
			u := x*f.u[0] + y*f.u[1] + z*f.u[2]
			v := x*f.v[0] + y*f.v[1] + z*f.v[2]
			return byte(7 + 2*u + 4*v)
		}
		ao, levels := sampleFaceLighting(0, 0, 0, litFace(face), air, light)
		for i, c := range f.corners {
			want := float32(7 + c[0] + 2*c[1])
			if ao[i] != 0 || levels[i] != want {
				t.Fatalf("face %d vertex %d: AO=%v light=%v want %v", face, i, ao[i], levels[i], want)
			}
		}
	}
}

func TestSmoothLightSharedVerticesAcrossBoundaries(t *testing.T) {
	initBlockRegistry()
	air := func(x, y, z int) byte { return blockAir }
	light := func(x, y, z int) byte { return byte((x*31 + y*13 + z*7) & 15) }
	for face, f := range faceLightLayouts {
		for _, offset := range []int{-1, 15} {
			p := [3]int{offset*f.u[0] + 4*f.v[0], offset*f.u[1] + 4*f.v[1], offset*f.u[2] + 4*f.v[2]}
			q := [3]int{p[0] + f.u[0], p[1] + f.u[1], p[2] + f.u[2]}
			shared := make(map[[3]int]float32)
			matches := 0
			for j, pos := range [][3]int{p, q} {
				_, levels := sampleFaceLighting(pos[0], pos[1], pos[2], litFace(face), air, light)
				for i, c := range f.corners {
					var vertex [3]int
					for axis := range vertex {
						vertex[axis] = 2*pos[axis] + f.normal[axis] + c[0]*f.u[axis] + c[1]*f.v[axis]
					}
					if old, ok := shared[vertex]; ok {
						matches++
						if old != levels[i] {
							t.Fatalf("seam at face %d coordinate %v", face, vertex)
						}
					}
					if j == 0 {
						shared[vertex] = levels[i]
					}
				}
			}
			if matches != 2 {
				t.Fatal("expected two shared edge vertices")
			}
		}
	}
}

func TestSmoothLightOccludedDiagonalDoesNotLeak(t *testing.T) {
	initBlockRegistry()
	for face, f := range faceLightLayouts {
		at := func(u, v int) [3]int {
			return [3]int{f.normal[0] + u*f.u[0] + v*f.v[0], f.normal[1] + u*f.u[1] + v*f.v[1], f.normal[2] + u*f.u[2] + v*f.v[2]}
		}
		s1, s2, diagonal := at(1, 0), at(0, 1), at(1, 1)
		blocks := func(x, y, z int) byte {
			p := [3]int{x, y, z}
			if p == s1 || p == s2 {
				return blockStone
			}
			return blockAir
		}
		light := func(x, y, z int) byte {
			if [3]int{x, y, z} == diagonal {
				return 15
			}
			return 0
		}
		ao, levels := sampleFaceLighting(0, 0, 0, litFace(face), blocks, light)
		for i, c := range f.corners {
			if c == [2]int{1, 1} && (ao[i] != 1 || levels[i] != 0) {
				t.Fatalf("corner leak on face %d: %v/%v", face, ao[i], levels[i])
			}
		}
		_, levels = sampleFaceLighting(0, 0, 0, litFace(face), blocks, func(int, int, int) byte { return 15 })
		for _, level := range levels {
			if level != 15 {
				t.Fatal("solid-cell zero light polluted interpolation")
			}
		}
	}
}

func TestSmoothColorEmissionAndAlpha(t *testing.T) {
	initBlockRegistry()
	a := &RenderAssets{}
	tints := make([]rl.Color, 4)
	ao := [4]float32{1, 1, 1, 1}
	c := a.applyAOSmooth(blockLava, rl.NewColor(140, 140, 140, 255), ao, [4]float32{}, tints)
	for _, v := range c {
		if v.R != 255 {
			t.Fatal("emissive face darkened")
		}
	}
	for i := range tints {
		tints[i] = rl.White
	}
	c = a.applyAOSmooth(blockWater, rl.White, ao, [4]float32{0, 5, 10, 15}, tints)
	for _, v := range c {
		if v.A != 200 {
			t.Fatal("light changed water alpha")
		}
	}
	if c[0].R >= c[3].R {
		t.Fatal("face still uses one light level")
	}
}

func TestSmoothQuadDiagonalPreservesWinding(t *testing.T) {
	for _, flip := range []bool{false, true} {
		colors := []rl.Color{rl.Black, rl.White, rl.Black, rl.White}
		if flip {
			colors = []rl.Color{rl.White, rl.Black, rl.White, rl.Black}
		}
		for _, f := range faceLightLayouts {
			var positions [12]float32
			for i, c := range f.corners {
				for axis := 0; axis < 3; axis++ {
					positions[i*3+axis] = float32(f.normal[axis]+c[0]*f.u[axis]+c[1]*f.v[axis]) * .5
				}
			}
			mb := new(meshBuilder)
			n := rl.NewVector3(float32(f.normal[0]), float32(f.normal[1]), float32(f.normal[2]))
			mb.addFaceSmooth(positions[:], n, make([]float32, 8), colors)
			if (mb.indices[2] == 3) != flip {
				t.Fatal("incorrect diagonal")
			}
			vertex := func(i uint32) rl.Vector3 { return rl.NewVector3(positions[i*3], positions[i*3+1], positions[i*3+2]) }
			for i := 0; i < 6; i += 3 {
				p, q, r := vertex(mb.indices[i]), vertex(mb.indices[i+1]), vertex(mb.indices[i+2])
				cross := rl.Vector3CrossProduct(rl.Vector3Subtract(q, p), rl.Vector3Subtract(r, p))
				if rl.Vector3DotProduct(cross, n) <= 0 {
					t.Fatal("diagonal reversed face winding")
				}
			}
		}
	}
}
