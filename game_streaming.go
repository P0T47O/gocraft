package main

import (
	"math"
	"time"
)

// Client-Pull: Request chunks we need but don't have.
// The caller supplies the renderer-neutral player/camera position so streaming
// no longer reaches through the global Raylib camera.
func requestMissingChunks(pos gameVec3) {
	if client == nil || world == nil {
		return
	}

	cx := int(math.Floor(float64(pos.X) / 16.0))
	cz := int(math.Floor(float64(pos.Z) / 16.0))
	renderRadius := renderDistance()
	maxRequestsPerFrame := 32 // Increased for faster loading

	requestCount := 0
	radiusSq := renderRadius * renderRadius

	requestChunk := func(dx, dz int) bool {
		if dx*dx+dz*dz > radiusSq {
			return true // continue, not counted
		}
		chunkX, chunkZ := cx+dx, cz+dz
		key := chunkKey{X: chunkX, Z: chunkZ}
		chunk := world.getChunkIfGenerated(chunkX, chunkZ)
		if chunk != nil && chunk.generated {
			return true
		}
		if sent, pending := pendingChunkRequests[key]; pending && time.Since(sent) < 3*time.Second {
			return true
		}
		pendingChunkRequests[key] = time.Now()
		client.Send(&PacketChunkRequest{CX: int32(chunkX), CZ: int32(chunkZ)})
		requestCount++
		return requestCount < maxRequestsPerFrame
	}

	// Spiral out from center — only iterate ring edges
	if requestChunk(0, 0) {
		for r := 1; r <= renderRadius && requestCount < maxRequestsPerFrame; r++ {
			// Top and bottom edges
			for dx := -r; dx <= r && requestCount < maxRequestsPerFrame; dx++ {
				if !requestChunk(dx, -r) {
					break
				}
				if !requestChunk(dx, r) {
					break
				}
			}
			// Left and right edges (excluding corners already done)
			for dz := -r + 1; dz <= r-1 && requestCount < maxRequestsPerFrame; dz++ {
				if !requestChunk(-r, dz) {
					break
				}
				if !requestChunk(r, dz) {
					break
				}
			}
		}
	}
}
