package main

import "github.com/go-gl/mathgl/mgl32"

func meshVec3(x, y, z float32) gameVec3 { return gameVec3{X: x, Y: y, Z: z} }
func meshCross(a, b gameVec3) gameVec3 {
	return meshVec3(a.Y*b.Z-a.Z*b.Y, a.Z*b.X-a.X*b.Z, a.X*b.Y-a.Y*b.X)
}
func meshRotation(axis gameVec3, angle float32) mgl32.Mat4 {
	axis = gameVec3Normalize(axis)
	return mgl32.HomogRotate3D(angle, mgl32.Vec3{axis.X, axis.Y, axis.Z})
}
func meshTransform(v gameVec3, m mgl32.Mat4) gameVec3 {
	p := m.Mul4x1(mgl32.Vec4{v.X, v.Y, v.Z, 1})
	return meshVec3(p[0], p[1], p[2])
}
