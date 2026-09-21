package main

func resolveCollision(world *World, pos, delta gameVec3) gameVec3 {
	feet := pos
	feet.Y -= playerEyeY
	result := moveColliderCore(world, feet, delta, Collider{Width: playerRadius * 2, Depth: playerRadius * 2, Height: playerHeight})
	result.Y += playerEyeY
	return result
}
func collides(world *World, pos gameVec3) bool {
	pos.Y -= playerEyeY
	return colliderHitsCore(world, pos, Collider{Width: playerRadius * 2, Depth: playerRadius * 2, Height: playerHeight})
}
