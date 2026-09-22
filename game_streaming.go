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
	valid   bool
	keys    []chunkKey
	next    int
	refresh time.Time
}

var chunkRequests chunkRequestPlan

func (p *chunkRequestPlan) prepare(center chunkKey, radius int, now time.Time) {
	if !p.valid || p.center != center || p.radius != radius {
		// Translation preserves distance order. Walking across a chunk boundary
		// must not sort thousands of coordinates again at the same view radius.
		if p.valid && p.radius == radius {
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
		p.center, p.radius, p.valid = center, radius, true
		p.keys = p.keys[:0]
		for x := -radius; x <= radius; x++ {
			for z := -radius; z <= radius; z++ {
				if x*x+z*z <= radius*radius {
					p.keys = append(p.keys, chunkKey{center.X + x, center.Z + z})
				}
			}
		}
		sort.SliceStable(p.keys, func(i, j int) bool {
			a, b := p.keys[i], p.keys[j]
			ax, az, bx, bz := a.X-center.X, a.Z-center.Z, b.X-center.X, b.Z-center.Z
			return ax*ax+az*az < bx*bx+bz*bz
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

// Explicit clock keeps retry and teleport regressions deterministic.
func requestMissingChunksAt(pos gameVec3, now time.Time) {
	if client == nil || world == nil {
		return
	}
	center := chunkKey{int(math.Floor(float64(pos.X) / 16)), int(math.Floor(float64(pos.Z) / 16))}
	changed := !chunkRequests.valid || chunkRequests.center != center || chunkRequests.radius != renderDistance()
	chunkRequests.prepare(center, renderDistance(), now)
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
