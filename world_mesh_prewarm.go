package main

import "time"

type meshPrewarmSchedule struct {
	center           chunkKey
	radius, next     int
	valid, interiors bool
}

// Use spare queue capacity after visible work. Surface sections in every
// direction come first; a second sweep warms interiors without hiding caves.
func (w *World) prewarmMeshes(cx, cz, radius int, assets *RenderAssets, deadline time.Time) {
	s := &w.render.prewarm
	if !s.valid || s.center != (chunkKey{cx, cz}) || s.radius != radius {
		*s = meshPrewarmSchedule{center: chunkKey{cx, cz}, radius: radius, valid: true}
	}
	offsets := w.render.radiusOffsets(radius)
	jobs := 0
	for scanned := 0; scanned < 64 && time.Now().Before(deadline); scanned++ {
		// Reserve three quarters of the normal queue for newly visible work.
		if jobs >= 8 || len(w.meshJobs) >= 8 || len(w.meshResults) >= 8 {
			return
		}
		if s.next >= len(offsets) {
			s.next = 0
			s.interiors = !s.interiors
		}
		o := offsets[s.next]
		x, z := cx+o.dx, cz+o.dz
		c := w.getChunkIfGenerated(x, z)
		if c == nil {
			s.next++
			continue
		}
		ensureChunkSections(c)
		low, high := chunkHeight, 0
		for x := range c.heightMap {
			for _, h := range c.heightMap[x] {
				low = min(low, int(h))
				high = max(high, int(h))
			}
		}
		first, last := max(0, (high-1)/sectionHeight), max(0, (low-1)/sectionHeight-1)
		if s.interiors {
			first, last = sectionCount-1, 0
		}
		for sec := first; sec >= last; sec-- {
			if !c.sectionDirty[sec] || c.pendingOpaque[sec] || c.sectionBlocks[sec] == 0 || c.meshRetries[sec] > 5 {
				continue
			}
			if distantBuriedSection(int16(low), sec, float32(o.dx*o.dx+o.dz*o.dz)*chunkWidth*chunkWidth) {
				continue
			}
			if jobs >= 8 || len(w.meshJobs) >= 8 || !time.Now().Before(deadline) {
				return
			}
			if w.submitMesh(x, z, sec, assets) {
				jobs++
			}
		}
		s.next++
	}
}
