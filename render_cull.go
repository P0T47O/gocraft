package main

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

type Frustum struct {
	Planes [6]mgl32.Vec4 // x, y, z, w (distance)
}

func ExtractFrustum(vp mgl32.Mat4) Frustum {
	var f Frustum

	// Left
	f.Planes[0] = mgl32.Vec4{
		vp[3] + vp[0],
		vp[7] + vp[4],
		vp[11] + vp[8],
		vp[15] + vp[12],
	}
	// Right
	f.Planes[1] = mgl32.Vec4{
		vp[3] - vp[0],
		vp[7] - vp[4],
		vp[11] - vp[8],
		vp[15] - vp[12],
	}
	// Bottom
	f.Planes[2] = mgl32.Vec4{
		vp[3] + vp[1],
		vp[7] + vp[5],
		vp[11] + vp[9],
		vp[15] + vp[13],
	}
	// Top
	f.Planes[3] = mgl32.Vec4{
		vp[3] - vp[1],
		vp[7] - vp[5],
		vp[11] - vp[9],
		vp[15] - vp[13],
	}
	// Near
	f.Planes[4] = mgl32.Vec4{
		vp[3] + vp[2],
		vp[7] + vp[6],
		vp[11] + vp[10],
		vp[15] + vp[14],
	}
	// Far
	f.Planes[5] = mgl32.Vec4{
		vp[3] - vp[2],
		vp[7] - vp[6],
		vp[11] - vp[10],
		vp[15] - vp[14],
	}

	// Normalize planes
	for i := 0; i < 6; i++ {
		len := float32(math.Sqrt(float64(f.Planes[i].X()*f.Planes[i].X() + f.Planes[i].Y()*f.Planes[i].Y() + f.Planes[i].Z()*f.Planes[i].Z())))
		f.Planes[i] = f.Planes[i].Mul(1.0 / len)
	}

	return f
}

// IntersectsAABB returns true if the AABB is inside or partially inside the frustum.
// min and max are the corners of the AABB.
func (f *Frustum) IntersectsAABB(min, max mgl32.Vec3) bool {
	for i := 0; i < 6; i++ {
		// Find the point on the AABB closest to the "negative" side of the plane
		// (The "negative" side is outside the frustum)
		var p mgl32.Vec3
		if f.Planes[i].X() > 0 {
			p[0] = max[0]
		} else {
			p[0] = min[0]
		}
		if f.Planes[i].Y() > 0 {
			p[1] = max[1]
		} else {
			p[1] = min[1]
		}
		if f.Planes[i].Z() > 0 {
			p[2] = max[2]
		} else {
			p[2] = min[2]
		}

		// Dot product + w (distance)
		dist := f.Planes[i].X()*p.X() + f.Planes[i].Y()*p.Y() + f.Planes[i].Z()*p.Z() + f.Planes[i].W()

		// If the "closest" point is behind the plane, the whole AABB is outside
		if dist < 0 {
			return false
		}
	}
	return true
}
