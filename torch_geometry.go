package main

// torchGeometry describes the two cuboids used by both the world mesh and
// selection ray. Coordinates are relative to the center of the block.
type torchGeometry struct {
	axis, offset, stemSize, flameSize, flameOffset gameVec3
	angle                                          float32
}

func geometryForTorch(meta byte) torchGeometry {
	g := torchGeometry{
		axis: meshVec3(0, 1, 0), offset: meshVec3(0, -0.1875, 0),
		stemSize:    meshVec3(0.125, 0.625, 0.125),
		flameSize:   meshVec3(0.25, 0.375, 0.25),
		flameOffset: meshVec3(0, 0.5, 0),
	}
	if meta < 1 || meta > 4 {
		return g
	}
	g.offset.Y = -0.05
	g.stemSize.Y = 0.5
	g.flameSize.Y = 0.25
	g.flameOffset.Y = 0.375
	g.angle = 22.5 * 3.141592653589793 / 180
	const wallOffset = 0.34658
	var direction gameVec3
	switch meta {
	case 1: // Supported by the wall at Z+.
		g.offset.Z = wallOffset
		direction = meshVec3(0, 0, -1)
	case 2:
		g.offset.Z = -wallOffset
		direction = meshVec3(0, 0, 1)
	case 3:
		g.offset.X = wallOffset
		direction = meshVec3(-1, 0, 0)
	case 4:
		g.offset.X = -wallOffset
		direction = meshVec3(1, 0, 0)
	}
	g.axis = meshCross(meshVec3(0, 1, 0), direction)
	return g
}
