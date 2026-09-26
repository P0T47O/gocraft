package main

import "image/color"

// Partial blocks share one box description with physics and targeting. This
// path runs only for slabs/stairs, leaving the full-cube hot loop untouched.
func (a *RenderAssets) emitShapedBlock(id, meta byte, x, y, z int, textures blockFaces, getBlock BlockGetter, getMeta MetaGetter, getLight, getBlockLight LightGetter, getBuilder func(string, string) *meshBuilder) {
	boxes, count := shapeBoxesAt(id, meta, x, y, z, getBlock, getMeta)
	if id == blockWoodDoor && meta&shapeUpper != 0 {
		textures = blockFaces{Top: "textures/block/oak_door_top.png", Bottom: "textures/block/oak_door_top.png", North: "textures/block/oak_door_top.png", South: "textures/block/oak_door_top.png", East: "textures/block/oak_door_top.png", West: "textures/block/oak_door_top.png"}
	}
	paths := [6]string{textures.Top, textures.Bottom, textures.North, textures.South, textures.East, textures.West}
	meshLayer := "opaque"
	if id == blockWoodDoor {
		meshLayer = "cutout"
	}
	normals := [6]gameVec3{meshVec3(0, 1, 0), meshVec3(0, -1, 0), meshVec3(0, 0, -1), meshVec3(0, 0, 1), meshVec3(1, 0, 0), meshVec3(-1, 0, 0)}
	shade := [6]color.RGBA{meshColor(255, 255, 255, 255), meshColor(140, 140, 140, 255), meshColor(210, 210, 210, 255), meshColor(225, 225, 225, 255), meshColor(190, 190, 190, 255), meshColor(200, 200, 200, 255)}
	light, blockLight := getLight(x, y, z), getBlockLight(x, y, z)
	for boxIndex, box := range boxes[:count] {
		lx, ly, lz := box.minX, box.minY, box.minZ
		hx, hy, hz := box.maxX, box.maxY, box.maxZ
		verts := [6][12]float32{
			{lx, hy, hz, hx, hy, hz, hx, hy, lz, lx, hy, lz},
			{lx, ly, hz, lx, ly, lz, hx, ly, lz, hx, ly, hz},
			{lx, hy, lz, hx, hy, lz, hx, ly, lz, lx, ly, lz},
			{hx, hy, hz, lx, hy, hz, lx, ly, hz, hx, ly, hz},
			{hx, hy, lz, hx, hy, hz, hx, ly, hz, hx, ly, lz},
			{lx, hy, hz, lx, hy, lz, lx, ly, lz, lx, ly, hz},
		}
		for face := 0; face < 6; face++ {
			// The riser rests against the base. Its joining face is internal.
			if isStair(id) && boxIndex > 0 && (meta&shapeUpper == 0 && face == 1 || meta&shapeUpper != 0 && face == 0) {
				continue
			}
			if boxIndex == 2 && (meta&3 == faceNorth && face == 3 || meta&3 == faceSouth && face == 2 || meta&3 == faceEast && face == 5 || meta&3 == faceWest && face == 4) {
				continue
			}
			neighborX, neighborY, neighborZ := x, y, z
			external := false
			switch face {
			case 0:
				neighborY, external = y+1, hy == .5
			case 1:
				neighborY, external = y-1, ly == -.5
			case 2:
				neighborZ, external = z-1, lz == -.5
			case 3:
				neighborZ, external = z+1, hz == .5
			case 4:
				neighborX, external = x+1, hx == .5
			case 5:
				neighborX, external = x-1, lx == -.5
			}
			if external && isOpaqueBlock(getBlock(neighborX, neighborY, neighborZ)) {
				continue
			}
			path := paths[face]
			rect, atlased := a.getAtlasUV(path)
			usePath := path
			if atlased {
				usePath = "atlas"
			}
			v := make([]float32, 12)
			copy(v, verts[face][:])
			uv := make([]float32, 8)
			for i := 0; i < 4; i++ {
				xv, yv, zv := v[i*3], v[i*3+1], v[i*3+2]
				u, vv := xv+.5, zv+.5
				if face == 1 {
					u = .5 - xv
				} else if face == 2 || face == 3 {
					if face == 3 {
						u = .5 - xv
					}
					vv = .5 - yv
				} else if face == 4 || face == 5 {
					u = zv + .5
					if face == 5 {
						u = .5 - zv
					}
					vv = .5 - yv
				}
				if atlased {
					u = rect.X + u*rect.Width
					vv = rect.Y + vv*rect.Height
				}
				uv[i*2], uv[i*2+1] = u, vv
				v[i*3] += float32(x)
				v[i*3+1] += float32(y)
				v[i*3+2] += float32(z)
			}
			col := a.applyAO(id, shade[face], 0, false, light, color.RGBA{}, blockLight)
			getBuilder(meshLayer, usePath).addFace(v, normals[face], uv, col)
		}
	}
}
