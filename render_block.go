package main

import (
	"gocraft/platform"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) drawBlock(block byte, pos rl.Vector3, getBlock BlockGetter, getLight LightGetter, getMeta MetaGetter, x, y, z int) {
	waterTint := rl.NewColor(64, 120, 255, 200)

	topTint := rl.NewColor(255, 255, 255, 255)
	northTint := rl.NewColor(210, 210, 210, 255)
	southTint := rl.NewColor(225, 225, 225, 255)
	westTint := rl.NewColor(200, 200, 200, 255)
	eastTint := rl.NewColor(190, 190, 190, 255)
	bottomTint := rl.NewColor(140, 140, 140, 255)
	grassTint := rl.NewColor(112, 185, 70, 255)
	leafTint := rl.NewColor(96, 165, 60, 255)

	def := GetBlock(block)
	faces := def.Textures

	applyBiomeTint := func(col rl.Color, useGrass bool) rl.Color {
		if block == blockWater {
			return rl.NewColor(
				uint8(float32(col.R)*float32(waterTint.R)/255.0),
				uint8(float32(col.G)*float32(waterTint.G)/255.0),
				uint8(float32(col.B)*float32(waterTint.B)/255.0),
				waterTint.A,
			)
		}
		if block == blockLeaves {
			return rl.NewColor(
				uint8(float32(col.R)*float32(leafTint.R)/255.0),
				uint8(float32(col.G)*float32(leafTint.G)/255.0),
				uint8(float32(col.B)*float32(leafTint.B)/255.0),
				col.A,
			)
		}
		if block == blockGrass && useGrass {
			return rl.NewColor(
				uint8(float32(col.R)*float32(grassTint.R)/255.0),
				uint8(float32(col.G)*float32(grassTint.G)/255.0),
				uint8(float32(col.B)*float32(grassTint.B)/255.0),
				col.A,
			)
		}
		return col
	}

	isTransparent := func(b byte) bool {
		return GetBlock(b).IsTransparent
	}

	isOccluding := func(ix, iy, iz int) bool {
		b := getBlock(ix, iy, iz)
		return b != blockAir && !isTransparent(b)
	}

	shouldDrawFace := func(nx, ny, nz int) bool {
		neighbor := getBlock(nx, ny, nz)
		if neighbor == blockAir {
			return true
		}
		if isTransparent(neighbor) {
			// Ice and Water seamless logic
			if block == blockIce {
				if neighbor == blockIce || neighbor == blockWater {
					return false
				}
				return true
			}
			if block == blockWater {
				if neighbor == blockWater || neighbor == blockIce {
					return false
				}
				return true
			}
			// Glass/Leaves: User requested NO culling
			return true
		}
		return false
	}

	cornerAO := func(side1, side2, corner bool) int {
		if side1 && side2 {
			return 3
		}
		occlusion := 0
		if side1 {
			occlusion++
		}
		if side2 {
			occlusion++
		}
		if corner {
			occlusion++
		}
		return occlusion
	}

	applyAO := func(col rl.Color, ao float32, useGrass bool, light byte) rl.Color {
		col = applyBiomeTint(col, useGrass)
		f := 1.0 - ao*0.6
		if f < 0.4 {
			f = 0.4
		}
		lightF := 0.1 + (float32(light)/15.0)*0.9
		f *= lightF
		return rl.NewColor(
			uint8(float32(col.R)*f),
			uint8(float32(col.G)*f),
			uint8(float32(col.B)*f),
			col.A,
		)
	}

	aoTop := func() float32 {
		sx0 := isOccluding(x-1, y+1, z)
		sx1 := isOccluding(x+1, y+1, z)
		sz0 := isOccluding(x, y+1, z-1)
		sz1 := isOccluding(x, y+1, z+1)
		c00 := isOccluding(x-1, y+1, z-1)
		c01 := isOccluding(x-1, y+1, z+1)
		c10 := isOccluding(x+1, y+1, z-1)
		c11 := isOccluding(x+1, y+1, z+1)
		ao := cornerAO(sx0, sz0, c00) + cornerAO(sx0, sz1, c01) + cornerAO(sx1, sz0, c10) + cornerAO(sx1, sz1, c11)
		return float32(ao) / 12.0
	}

	aoBottom := func() float32 {
		sx0 := isOccluding(x-1, y-1, z)
		sx1 := isOccluding(x+1, y-1, z)
		sz0 := isOccluding(x, y-1, z-1)
		sz1 := isOccluding(x, y-1, z+1)
		c00 := isOccluding(x-1, y-1, z-1)
		c01 := isOccluding(x-1, y-1, z+1)
		c10 := isOccluding(x+1, y-1, z-1)
		c11 := isOccluding(x+1, y-1, z+1)
		ao := cornerAO(sx0, sz0, c00) + cornerAO(sx0, sz1, c01) + cornerAO(sx1, sz0, c10) + cornerAO(sx1, sz1, c11)
		return float32(ao) / 12.0
	}

	aoNorth := func() float32 {
		sx0 := isOccluding(x-1, y, z-1)
		sx1 := isOccluding(x+1, y, z-1)
		sy0 := isOccluding(x, y-1, z-1)
		sy1 := isOccluding(x, y+1, z-1)
		c00 := isOccluding(x-1, y-1, z-1)
		c01 := isOccluding(x-1, y+1, z-1)
		c10 := isOccluding(x+1, y-1, z-1)
		c11 := isOccluding(x+1, y+1, z-1)
		ao := cornerAO(sx0, sy0, c00) + cornerAO(sx0, sy1, c01) + cornerAO(sx1, sy0, c10) + cornerAO(sx1, sy1, c11)
		return float32(ao) / 12.0
	}

	aoSouth := func() float32 {
		sx0 := isOccluding(x-1, y, z+1)
		sx1 := isOccluding(x+1, y, z+1)
		sy0 := isOccluding(x, y-1, z+1)
		sy1 := isOccluding(x, y+1, z+1)
		c00 := isOccluding(x-1, y-1, z+1)
		c01 := isOccluding(x-1, y+1, z+1)
		c10 := isOccluding(x+1, y-1, z+1)
		c11 := isOccluding(x+1, y+1, z+1)
		ao := cornerAO(sx0, sy0, c00) + cornerAO(sx0, sy1, c01) + cornerAO(sx1, sy0, c10) + cornerAO(sx1, sy1, c11)
		return float32(ao) / 12.0
	}

	aoWest := func() float32 {
		sz0 := isOccluding(x-1, y, z-1)
		sz1 := isOccluding(x-1, y, z+1)
		sy0 := isOccluding(x-1, y-1, z)
		sy1 := isOccluding(x-1, y+1, z)
		c00 := isOccluding(x-1, y-1, z-1)
		c01 := isOccluding(x-1, y+1, z-1)
		c10 := isOccluding(x-1, y-1, z+1)
		c11 := isOccluding(x-1, y+1, z+1)
		ao := cornerAO(sz0, sy0, c00) + cornerAO(sz0, sy1, c01) + cornerAO(sz1, sy0, c10) + cornerAO(sz1, sy1, c11)
		return float32(ao) / 12.0
	}

	aoEast := func() float32 {
		sz0 := isOccluding(x+1, y, z-1)
		sz1 := isOccluding(x+1, y, z+1)
		sy0 := isOccluding(x+1, y-1, z)
		sy1 := isOccluding(x+1, y+1, z)
		c00 := isOccluding(x+1, y-1, z-1)
		c01 := isOccluding(x+1, y+1, z-1)
		c10 := isOccluding(x+1, y-1, z+1)
		c11 := isOccluding(x+1, y+1, z+1)
		ao := cornerAO(sz0, sy0, c00) + cornerAO(sz0, sy1, c01) + cornerAO(sz1, sy0, c10) + cornerAO(sz1, sy1, c11)
		return float32(ao) / 12.0
	}

	lightTop := getLight(x, y+1, z)
	lightBottom := getLight(x, y-1, z)
	lightNorth := getLight(x, y, z-1)
	lightSouth := getLight(x, y, z+1)
	lightWest := getLight(x-1, y, z)
	lightEast := getLight(x+1, y, z)

	if def.RenderType == RenderTypeCross {
		light := getLight(x, y, z)
		rl.DisableBackfaceCulling()
		// Plane 1 - Both sides
		rl.DrawModel(a.getFaceModelAnimated("cross_1", faces.North), pos, 1, applyAO(topTint, 0, false, light))
		// Note: Immediate mode DisableBackfaceCulling already shows the face from both sides,
		// but chunk meshes need explicit geometry. We stick to this for simplicity here.
		rl.DrawModel(a.getFaceModelAnimated("cross_2", faces.North), pos, 1, applyAO(topTint, 0, false, light))
		rl.EnableBackfaceCulling()
		return
	}

	if def.RenderType == RenderTypeTorch {
		light := getLight(x, y, z)
		stemScale := rl.NewVector3(0.125, 0.625, 0.125)
		stemPos := pos
		stemPos.Y -= 0.04
		tiltAxis := rl.NewVector3(0, 1, 0)
		tiltDeg := float32(0)
		wallOffset := float32(0.4375)
		tiltAngle := float32(22.5)
		switch getMeta(x, y, z) {
		case 1:
			dir := rl.NewVector3(0, 0, -1)
			stemPos.Z += wallOffset
			tiltAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
			tiltDeg = tiltAngle
		case 2:
			dir := rl.NewVector3(0, 0, 1)
			stemPos.Z -= wallOffset
			tiltAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
			tiltDeg = tiltAngle
		case 3:
			dir := rl.NewVector3(-1, 0, 0)
			stemPos.X += wallOffset
			tiltAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
			tiltDeg = tiltAngle
		case 4:
			dir := rl.NewVector3(1, 0, 0)
			stemPos.X -= wallOffset
			tiltAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
			tiltDeg = tiltAngle
		}

		stemModelNorth := a.getFaceModelAnimated("torch_side_north", faces.North)
		stemModelSouth := a.getFaceModelAnimated("torch_side_south", faces.North)
		stemModelEast := a.getFaceModelAnimated("torch_side_east", faces.North)
		stemModelWest := a.getFaceModelAnimated("torch_side_west", faces.North)
		stemModelTop := a.getFaceModelAnimated("torch_top", faces.Top)
		stemModelBottom := a.getFaceModelAnimated("torch_bottom", faces.Bottom)

		rl.DisableBackfaceCulling()
		rl.DrawModelEx(stemModelNorth, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(northTint, 0, false, light))
		rl.DrawModelEx(stemModelSouth, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(southTint, 0, false, light))
		rl.DrawModelEx(stemModelEast, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(eastTint, 0, false, light))
		rl.DrawModelEx(stemModelWest, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(westTint, 0, false, light))
		rl.DrawModelEx(stemModelTop, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(topTint, 0, false, light))
		rl.DrawModelEx(stemModelBottom, stemPos, tiltAxis, tiltDeg, stemScale, applyAO(bottomTint, 0, false, light))

		flameScale := rl.NewVector3(0.5, 0.5, 0.5)
		flamePos := stemPos
		flamePos.Y += stemScale.Y*0.5 + 0.125

		flameModelNorth := a.getFaceModelAnimated("torch_flame_north", faces.North)
		flameModelSouth := a.getFaceModelAnimated("torch_flame_south", faces.North)
		flameModelEast := a.getFaceModelAnimated("torch_flame_east", faces.North)
		flameModelWest := a.getFaceModelAnimated("torch_flame_west", faces.North)
		flameModelTop := a.getFaceModelAnimated("torch_flame_top", faces.Top)

		rl.DrawModelEx(flameModelNorth, flamePos, tiltAxis, tiltDeg, flameScale, applyAO(topTint, 0, false, light))
		rl.DrawModelEx(flameModelSouth, flamePos, tiltAxis, tiltDeg, flameScale, applyAO(topTint, 0, false, light))
		rl.DrawModelEx(flameModelEast, flamePos, tiltAxis, tiltDeg, flameScale, applyAO(topTint, 0, false, light))
		rl.DrawModelEx(flameModelWest, flamePos, tiltAxis, tiltDeg, flameScale, applyAO(topTint, 0, false, light))
		rl.DrawModelEx(flameModelTop, flamePos, tiltAxis, tiltDeg, flameScale, applyAO(topTint, 0, false, light))
		rl.EnableBackfaceCulling()
		return
	}

	if def.RenderType == RenderTypeCross {
		// Cross rendering (Flowers, Saplings, etc.)
		light := getLight(x, y, z)
		model1 := a.getFaceModelAnimated("cross_1", faces.North)
		model2 := a.getFaceModelAnimated("cross_2", faces.North)

		rl.DisableBackfaceCulling()
		rl.DrawModel(model1, pos, 1, applyAO(topTint, 0, false, light))
		rl.DrawModel(model2, pos, 1, applyAO(topTint, 0, false, light))
		rl.EnableBackfaceCulling()
		return
	}

	if block == blockCactus {
		// Special Cactus rendering (Inset)
		if shouldDrawFace(x, y+1, z) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_top", faces.Top), pos, 1, applyAO(topTint, aoTop(), false, lightTop))
		}
		if shouldDrawFace(x, y-1, z) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_bottom", faces.Bottom), pos, 1, applyAO(bottomTint, aoBottom(), false, lightBottom))
		}
		if shouldDrawFace(x, y, z-1) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_north", faces.North), pos, 1, applyAO(northTint, aoNorth(), false, lightNorth))
		}
		if shouldDrawFace(x, y, z+1) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_south", faces.South), pos, 1, applyAO(southTint, aoSouth(), false, lightSouth))
		}
		if shouldDrawFace(x-1, y, z) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_west", faces.West), pos, 1, applyAO(westTint, aoWest(), false, lightWest))
		}
		if shouldDrawFace(x+1, y, z) {
			rl.DrawModel(a.getFaceModelAnimated("cactus_east", faces.East), pos, 1, applyAO(eastTint, aoEast(), false, lightEast))
		}
		return
	}

	if shouldDrawFace(x, y+1, z) {
		rl.DrawModel(a.getFaceModelAnimated("top", faces.Top), pos, 1, applyAO(topTint, aoTop(), true, lightTop))
	}
	if shouldDrawFace(x, y-1, z) {
		rl.DrawModel(a.getFaceModelAnimated("bottom", faces.Bottom), pos, 1, applyAO(bottomTint, aoBottom(), false, lightBottom))
	}
	if shouldDrawFace(x, y, z-1) {
		if block == blockGrass {
			rl.DrawModel(a.getFaceModelAnimated("north_base", faces.Bottom), pos, 1, applyAO(northTint, aoNorth(), false, lightNorth))
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1.0, -1.0)
			rl.DrawModel(a.getFaceModelAnimated("north_overlay", "textures/block/grass_block_side_overlay.png"), pos, 1, applyAO(northTint, aoNorth(), true, lightNorth))
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
		} else {
			rl.DrawModel(a.getFaceModelAnimated("north", faces.North), pos, 1, applyAO(northTint, aoNorth(), false, lightNorth))
		}
	}
	if shouldDrawFace(x, y, z+1) {
		if block == blockGrass {
			rl.DrawModel(a.getFaceModelAnimated("south_base", faces.Bottom), pos, 1, applyAO(southTint, aoSouth(), false, lightSouth))
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1.0, -1.0)
			rl.DrawModel(a.getFaceModelAnimated("south_overlay", "textures/block/grass_block_side_overlay.png"), pos, 1, applyAO(southTint, aoSouth(), true, lightSouth))
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
		} else {
			rl.DrawModel(a.getFaceModelAnimated("south", faces.South), pos, 1, applyAO(southTint, aoSouth(), false, lightSouth))
		}
	}
	if shouldDrawFace(x-1, y, z) {
		if block == blockGrass {
			rl.DrawModel(a.getFaceModelAnimated("west_base", faces.Bottom), pos, 1, applyAO(westTint, aoWest(), false, lightWest))
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1.0, -1.0)
			rl.DrawModel(a.getFaceModelAnimated("west_overlay", "textures/block/grass_block_side_overlay.png"), pos, 1, applyAO(westTint, aoWest(), true, lightWest))
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
		} else {
			rl.DrawModel(a.getFaceModelAnimated("west", faces.West), pos, 1, applyAO(westTint, aoWest(), false, lightWest))
		}
	}
	if shouldDrawFace(x+1, y, z) {
		if block == blockGrass {
			rl.DrawModel(a.getFaceModelAnimated("east_base", faces.Bottom), pos, 1, applyAO(eastTint, aoEast(), false, lightEast))
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1.0, -1.0)
			rl.DrawModel(a.getFaceModelAnimated("east_overlay", "textures/block/grass_block_side_overlay.png"), pos, 1, applyAO(eastTint, aoEast(), true, lightEast))
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
		} else {
			rl.DrawModel(a.getFaceModelAnimated("east", faces.East), pos, 1, applyAO(eastTint, aoEast(), false, lightEast))
		}
	}
}
