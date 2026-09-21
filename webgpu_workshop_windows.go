//go:build windows

package main

// Draw the workshop ground grid and world-axis collision wireframe using thin
// boxes, so diagnostic geometry shares the normal WebGPU depth and atlas path.
func (b *webGPUEntityBatch) addWorkshopGuides() {
	u0, v0, u1, v1, ok := webGPUEntityUV(GetBlock(blockIronBlock).Textures.Top)
	if !ok {
		return
	}
	uv := [4]float32{u0, v0, u1, v1}
	line := func(x, y, z, sx, sy, sz float32, c [4]uint8) { b.addBox(x, y, z, sx, sy, sz, 0, uv, c) }
	for n := -6; n <= 6; n++ {
		line(float32(n), -.025, 0, .008, .008, 12, [4]uint8{110, 130, 130, 255})
		line(0, -.025, float32(n), 12, .008, .008, [4]uint8{110, 130, 130, 255})
	}
	if !mobWorkshopBounds {
		return
	}
	for _, e := range remoteEntities {
		d, ok := mobContent.Definitions[e.MobKind]
		if !ok {
			continue
		}
		x, y, z := float32(e.X), float32(e.Y), float32(e.Z)
		hx, hz, h := d.Collider.Width/2, d.Collider.Depth/2, d.Collider.Height
		color := [4]uint8{80, 255, 80, 255}
		for _, a := range []float32{-1, 1} {
			for _, c := range []float32{-1, 1} {
				line(x+a*hx, y+h/2, z+c*hz, .012, h, .012, color)
			}
			for _, dy := range []float32{0, h} {
				line(x, y+dy, z+a*hz, hx*2, .012, .012, color)
				line(x+a*hx, y+dy, z, .012, .012, hz*2, color)
			}
		}
	}
}
