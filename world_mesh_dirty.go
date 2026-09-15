package main

// Coalesce updates until a fresh snapshot is submitted. A running snapshot is
// invalidated once; subsequent edits are folded into the eventual replacement.
// Caller owns the chunk and has initialized its sections.
func (c *Chunk) invalidateMeshSection(sec int) {
	if c.sectionDirty[sec] && c.meshVersion[sec] != 0 &&
		(!c.pendingOpaque[sec] || c.meshVersion[sec] != c.meshSubmittedVersion[sec]) {
		return
	}
	c.sectionDirty[sec] = true
	c.meshVersion[sec]++
	c.meshRetries[sec] = 0
}

// A voxel affects its own section and only neighbors whose one-voxel sampling
// halo includes it. Bit masks merge thousands of voxel changes without maps.
type chunkMeshChanges [3][3]uint16

func (m *chunkMeshChanges) mark(x, y, z int) {
	loX, hiX, loZ, hiZ := 1, 1, 1, 1
	if x == 0 {
		loX = 0
	}
	if x == chunkWidth-1 {
		hiX = 2
	}
	if z == 0 {
		loZ = 0
	}
	if z == chunkWidth-1 {
		hiZ = 2
	}
	loSec, hiSec := max(0, y-1)/sectionHeight, min(chunkHeight-1, y+1)/sectionHeight
	var bits uint16
	for sec := loSec; sec <= hiSec; sec++ {
		bits |= 1 << sec
	}
	for dx := loX; dx <= hiX; dx++ {
		for dz := loZ; dz <= hiZ; dz++ {
			m[dx][dz] |= bits
		}
	}
}

func (m *chunkMeshChanges) apply(w *World, cx, cz int) {
	for dx := 0; dx < 3; dx++ {
		for dz := 0; dz < 3; dz++ {
			bits := m[dx][dz]
			if bits == 0 {
				continue
			}
			c := w.getChunkIfGenerated(cx+dx-1, cz+dz-1)
			if c == nil {
				continue
			}
			ensureChunkSections(c)
			for sec := 0; sec < sectionCount; sec++ {
				if bits&(1<<sec) != 0 {
					c.invalidateMeshSection(sec)
				}
			}
		}
	}
}
