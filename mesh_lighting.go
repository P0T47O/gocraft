package main

import rl "github.com/gen2brain/raylib-go/raylib"

type litFace int

const (
	lightTop litFace = iota
	lightBottom
	lightNorth
	lightSouth
	lightEast
	lightWest
)

// Tangents and corner signs follow the actual quad vertex order, not UV order.
type faceLightLayout struct {
	normal, u, v [3]int
	corners      [4][2]int
}

var faceLightLayouts = [...]faceLightLayout{
	{[3]int{0, 1, 0}, [3]int{1, 0, 0}, [3]int{0, 0, 1}, [4][2]int{{-1, 1}, {1, 1}, {1, -1}, {-1, -1}}},
	{[3]int{0, -1, 0}, [3]int{1, 0, 0}, [3]int{0, 0, 1}, [4][2]int{{-1, 1}, {-1, -1}, {1, -1}, {1, 1}}},
	{[3]int{0, 0, -1}, [3]int{1, 0, 0}, [3]int{0, 1, 0}, [4][2]int{{-1, 1}, {1, 1}, {1, -1}, {-1, -1}}},
	{[3]int{0, 0, 1}, [3]int{1, 0, 0}, [3]int{0, 1, 0}, [4][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}}},
	{[3]int{1, 0, 0}, [3]int{0, 0, 1}, [3]int{0, 1, 0}, [4][2]int{{-1, 1}, {1, 1}, {1, -1}, {-1, -1}}},
	{[3]int{-1, 0, 0}, [3]int{0, 0, 1}, [3]int{0, 1, 0}, [4][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}}},
}

// Sample the 3x3 plane immediately outside the face once. Each vertex shares
// the four touching cells with adjacent coplanar faces, including chunk seams.
// Opaque cells are excluded, not averaged as black. Two blocked sides prevent
// a diagonal light leak; AO remains a separate geometric visibility factor.
func sampleFaceLighting(x, y, z int, face litFace, getBlock BlockGetter, getLight LightGetter) (aos, lights [4]float32) {
	f := faceLightLayouts[face]
	var blocked [9]bool
	var levels [9]float32
	for v := -1; v <= 1; v++ {
		for u := -1; u <= 1; u++ {
			i := (v+1)*3 + u + 1
			wx, wy, wz := x+f.normal[0]+u*f.u[0]+v*f.v[0], y+f.normal[1]+u*f.u[1]+v*f.v[1], z+f.normal[2]+u*f.u[2]+v*f.v[2]
			blocked[i] = GetBlock(getBlock(wx, wy, wz)).IsOpaque
			if !blocked[i] {
				levels[i] = float32(min(byte(15), getLight(wx, wy, wz)))
			}
		}
	}
	for i, corner := range f.corners {
		u, v := corner[0], corner[1]
		s1, s2, diag := 4+u, 4+v*3, 4+u+v*3
		oc := 0
		if blocked[s1] {
			oc++
		}
		if blocked[s2] {
			oc++
		}
		if blocked[diag] {
			oc++
		}
		if blocked[s1] && blocked[s2] {
			oc = 3
		}
		aos[i] = float32(oc) / 3
		sum, count := float32(0), float32(0)
		for _, idx := range [4]int{4, s1, s2, diag} {
			if blocked[idx] || (idx == diag && blocked[s1] && blocked[s2]) {
				continue
			}
			sum += levels[idx]
			count++
		}
		if count > 0 {
			lights[i] = sum / count
		}
	}
	return
}

func (a *RenderAssets) applyAOSmooth(block byte, col rl.Color, aos [4]float32, lights [4]float32, tints []rl.Color) []rl.Color {
	res := make([]rl.Color, 4)
	emission := float32(GetBlock(block).LightLevel)
	for i := range res {
		c := col
		if emission > 0 {
			c.R, c.G, c.B = 255, 255, 255
		}
		if tints[i].A > 0 {
			c.R = uint8(float32(c.R) * float32(tints[i].R) / 255)
			c.G = uint8(float32(c.G) * float32(tints[i].G) / 255)
			c.B = uint8(float32(c.B) * float32(tints[i].B) / 255)
			if block == blockWater {
				c.A = 200
			}
		}
		ao := aos[i]
		if block == blockGlass || block == blockIce || emission > 0 {
			ao = 0
		}
		f := (1 - 0.6*ao) * (0.1 + 0.9*max(emission, lights[i])/15)
		res[i] = rl.NewColor(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
	}
	return res
}

func flipLightDiagonal(colors []rl.Color) bool {
	// Compare luminance, leaving alpha (e.g. water transparency) out of lighting.
	luma := func(c rl.Color) int { return 54*int(c.R) + 183*int(c.G) + 19*int(c.B) }
	return luma(colors[0])+luma(colors[2]) > luma(colors[1])+luma(colors[3])
}
