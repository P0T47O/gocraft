package main

import "image/color"

// The vanilla-style bed side textures include transparent space above the
// mattress and cutouts for the legs. Keep their quads a full block high so
// those alpha shapes land at the intended 9/16-block mattress height.
func (a *RenderAssets) emitBedBlock(x, y, z int, meta byte, getBlock BlockGetter, getMeta MetaGetter, getLight, getBlockLight LightGetter, getBuilder func(string, string) *meshBuilder) {
	head := meta&shapeUpper != 0
	facing := meta & 3
	dx, dz := faceOffset(facing)
	rightX, rightZ := dz, -dx
	px, py, pz := float32(x), float32(y), float32(z)
	topPath := "textures/block/red_bed_foot_up.png"
	sidePrefix := "textures/block/red_bed_foot_"
	if head {
		topPath = "textures/block/red_bed_head_up.png"
		sidePrefix = "textures/block/red_bed_head_"
	}
	const mattressTop = float32(.0625)
	vertices := [6][12]float32{
		{-.5, mattressTop, .5, .5, mattressTop, .5, .5, mattressTop, -.5, -.5, mattressTop, -.5},
		{-.5, -.5, .5, -.5, -.5, -.5, .5, -.5, -.5, .5, -.5, .5},
		{-.5, .5, -.5, .5, .5, -.5, .5, -.5, -.5, -.5, -.5, -.5},
		{.5, .5, .5, -.5, .5, .5, -.5, -.5, .5, .5, -.5, .5},
		{.5, .5, -.5, .5, .5, .5, .5, -.5, .5, .5, -.5, -.5},
		{-.5, .5, .5, -.5, .5, -.5, -.5, -.5, -.5, -.5, -.5, .5},
	}
	normals := [6]gameVec3{meshVec3(0, 1, 0), meshVec3(0, -1, 0), meshVec3(0, 0, -1), meshVec3(0, 0, 1), meshVec3(1, 0, 0), meshVec3(-1, 0, 0)}
	shade := [6]color.RGBA{meshColor(255, 255, 255, 255), meshColor(140, 140, 140, 255), meshColor(210, 210, 210, 255), meshColor(225, 225, 225, 255), meshColor(190, 190, 190, 255), meshColor(200, 200, 200, 255)}
	light, blockLight := getLight(x, y, z), getBlockLight(x, y, z)
	for face := 0; face < 6; face++ {
		nx, ny, nz := x, y, z
		path := topPath
		switch face {
		case 0:
			ny++
		case 1:
			ny--
			path = "textures/block/bed_down.png"
		case 2:
			nz--
		case 3:
			nz++
		case 4:
			nx++
		case 5:
			nx--
		}
		if isOpaqueBlock(getBlock(nx, ny, nz)) {
			continue
		}
		if face >= 2 {
			faceDX, faceDZ := nx-x, nz-z
			if (head && faceDX == -dx && faceDZ == -dz || !head && faceDX == dx && faceDZ == dz) && getBlock(nx, ny, nz) == blockBed {
				otherMeta := getMeta(nx, ny, nz)
				if otherMeta&3 == facing && otherMeta&shapeUpper != meta&shapeUpper {
					continue
				}
			}
			switch {
			case faceDX == rightX && faceDZ == rightZ:
				path = sidePrefix + "east.png"
			case faceDX == -rightX && faceDZ == -rightZ:
				path = sidePrefix + "west.png"
			default:
				// The resource pack has one exposed-end texture, shared by
				// both ends of the assembled bed.
				path = "textures/block/red_bed_foot_south.png"
			}
		}
		uv := [8]float32{0, 0, 1, 0, 1, 1, 0, 1}
		if face == 0 {
			for i := 0; i < 4; i++ {
				vx, vz := vertices[face][i*3], vertices[face][i*3+2]
				uv[i*2] = .5 + vx*float32(rightX) + vz*float32(rightZ)
				uv[i*2+1] = .5 - vx*float32(dx) - vz*float32(dz)
			}
		} else if face == 1 {
			uv = [8]float32{0, 0, 0, 1, 1, 1, 1, 0}
		}
		rect, atlased := a.getAtlasUV(path)
		usePath := path
		if atlased {
			usePath = "atlas"
			for i := 0; i < 4; i++ {
				uv[i*2] = rect.X + uv[i*2]*rect.Width
				uv[i*2+1] = rect.Y + uv[i*2+1]*rect.Height
			}
		}
		v := make([]float32, 12)
		for i := 0; i < 4; i++ {
			v[i*3] = vertices[face][i*3] + px
			v[i*3+1] = vertices[face][i*3+1] + py
			v[i*3+2] = vertices[face][i*3+2] + pz
		}
		col := a.applyAO(blockBed, shade[face], 0, false, light, color.RGBA{}, blockLight)
		getBuilder("cutout", usePath).addFace(v, normals[face], uv[:], col)
	}
}
