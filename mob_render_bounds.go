package main

import rl "github.com/gen2brain/raylib-go/raylib"

// mobBox is presentation-only now; authoritative/server collision uses the
// renderer-neutral Collider math in mob_protocol.go and actor_physics.go.
func mobBox(pos rl.Vector3, c Collider) rl.BoundingBox {
	return rl.BoundingBox{
		Min: rl.NewVector3(pos.X-c.Width/2, pos.Y, pos.Z-c.Depth/2),
		Max: rl.NewVector3(pos.X+c.Width/2, pos.Y+c.Height, pos.Z+c.Depth/2),
	}
}
