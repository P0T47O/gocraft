package main

import (
	"math"
	"sort"
	"time"
)

// Main-loop-owned traversal, amortized across frames instead of restarting
// from the center. Stationary clients periodically revisit retries and unloads.
type chunkRequestPlan struct {
	center  chunkKey
	radius  int
	facing  int
	valid   bool
	keys    []chunkKey
	next    int
	refresh time.Time
}

var chunkRequests chunkRequestPlan

// Bound outstanding requests, not just packets waiting in the socket queue.
// Receipt frees a slot; retries reuse their existing slot.
const maxPendingChunkRequests = 64

func (p *chunkRequestPlan) prepare(center chunkKey, radius int, now time.Time) {
	p.prepareFacing(center, radius, -1, now)
}

// Keep nearby chunks omnidirectional, then favor the viewed half of each
// distance band. Re-sort only after a substantial turn, not every frame.
func (p *chunkRequestPlan) prepareFacing(center chunkKey, radius, facing int, now time.Time) {
	if !p.valid || p.center != center || p.radius != radius || p.facing != facing {
		// Translation preserves distance order. Walking across a chunk boundary
		// must not sort thousands of coordinates again at the same view radius.
		if p.valid && p.radius == radius && p.facing == facing {
			dx, dz := center.X-p.center.X, center.Z-p.center.Z
			for i := range p.keys {
				p.keys[i].X += dx
				p.keys[i].Z += dz
			}
			p.center = center
			p.next = 0
			p.refresh = now.Add(time.Second)
			return
		}
		p.center, p.radius, p.facing, p.valid = center, radius, facing, true
		p.keys = p.keys[:0]
		for x := -radius; x <= radius; x++ {
			for z := -radius; z <= radius; z++ {
				if x*x+z*z <= radius*radius {
					p.keys = append(p.keys, chunkKey{center.X + x, center.Z + z})
				}
			}
		}
		forwardX, forwardZ := 0.0, 0.0
		if facing >= 0 {
			angle := float64(facing) * math.Pi / 4
			forwardX, forwardZ = math.Sin(angle), math.Cos(angle)
		}
		sort.SliceStable(p.keys, func(i, j int) bool {
			a, b := p.keys[i], p.keys[j]
			ax, az, bx, bz := a.X-center.X, a.Z-center.Z, b.X-center.X, b.Z-center.Z
			ad, bd := ax*ax+az*az, bx*bx+bz*bz
			if facing >= 0 && (ad > 16 || bd > 16) {
				priority := func(x, z, distance int) int {
					if distance <= 16 {
						return 0
					}
					band := int(math.Sqrt(float64(distance))-1) / 4
					behind := 0
					if float64(x)*forwardX+float64(z)*forwardZ < 0 {
						behind = 1
					}
					return 1 + band*2 + behind
				}
				if ap, bp := priority(ax, az, ad), priority(bx, bz, bd); ap != bp {
					return ap < bp
				}
			}
			return ad < bd
		})
		p.next = 0
		p.refresh = now.Add(time.Second)
	} else if p.next == len(p.keys) && !now.Before(p.refresh) {
		p.next = 0
		p.refresh = now.Add(time.Second)
	}
}

func (p *chunkRequestPlan) contains(k chunkKey) bool {
	x, z := k.X-p.center.X, k.Z-p.center.Z
	return x*x+z*z <= p.radius*p.radius
}

func requestMissingChunks(pos gameVec3) {
	requestMissingChunksAt(pos, time.Now())
}

func requestMissingChunksFacing(pos gameVec3, yaw float32) {
	requestMissingChunksFacingAt(pos, yaw, time.Now())
}

// Explicit clock keeps retry and teleport regressions deterministic.
func requestMissingChunksAt(pos gameVec3, now time.Time) {
	requestMissingChunksWithFacing(pos, -1, now)
}

func requestMissingChunksFacingAt(pos gameVec3, yaw float32, now time.Time) {
	facing := int(math.Round(float64(yaw)/(math.Pi/4))) & 7
	requestMissingChunksWithFacing(pos, facing, now)
}

func requestMissingChunksWithFacing(pos gameVec3, facing int, now time.Time) {
	if client == nil || world == nil {
		return
	}
	center := chunkKey{int(math.Floor(float64(pos.X) / 16)), int(math.Floor(float64(pos.Z) / 16))}
	changed := !chunkRequests.valid || chunkRequests.center != center || chunkRequests.radius != renderDistance() || chunkRequests.facing != facing
	chunkRequests.prepareFacing(center, renderDistance(), facing, now)
	if changed {
		for key := range pendingChunkRequests {
			if !chunkRequests.contains(key) {
				delete(pendingChunkRequests, key)
			}
		}
		for key := range chunkLoadStarts {
			if !chunkRequests.contains(key) {
				delete(chunkLoadStarts, key)
			}
		}
	}
	// Reserve half the outgoing queue for gameplay. A retryable chunk request
	// must not enter Client.Send's disconnect-on-saturation path.
	for checked, sent := 0, 0; chunkRequests.next < len(chunkRequests.keys) && checked < 256 && sent < 32; checked++ {
		key := chunkRequests.keys[chunkRequests.next]
		if world.getChunkIfGenerated(key.X, key.Z) != nil {
			chunkRequests.next++
			continue
		}
		if last, ok := pendingChunkRequests[key]; ok && now.Sub(last) < 3*time.Second {
			chunkRequests.next++
			continue
		}
		if _, retry := pendingChunkRequests[key]; !retry && len(pendingChunkRequests) >= maxPendingChunkRequests {
			// Revisit pending entries so a dropped/rejected request can still be
			// retried when the window is full. Do not strand the cursor here.
			chunkRequests.next = 0
			return
		}
		if len(client.Outgoing) >= max(1, cap(client.Outgoing)/2) {
			return
		}
		select {
		case <-client.done:
			return
		default:
		}
		select {
		case client.Outgoing <- &PacketChunkRequest{CX: int32(key.X), CZ: int32(key.Z)}:
			pendingChunkRequests[key] = now
			if _, ok := chunkLoadStarts[key]; !ok {
				chunkLoadStarts[key] = now
			}
			chunkRequests.next++
			sent++
		default:
			return
		}
	}
}
