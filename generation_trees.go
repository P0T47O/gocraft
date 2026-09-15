package main

const treeRadius = 2

type treeAnchor struct {
	x, y, z, height int
	log, leaves     byte
	spruce          bool
}

func sampleTreeAnchor(seed uint32, x, z int) (treeAnchor, bool) {
	c := sampleTerrainColumn(seed, x, z)
	return treeAnchorFromColumn(seed, x, z, c)
}

func treeAnchorFromColumn(seed uint32, x, z int, c terrainColumn) (treeAnchor, bool) {
	a := treeAnchor{x: x, y: c.height, z: z, log: blockLog, leaves: blockLeaves}
	if c.height < seaLevel || c.height >= chunkHeight-20 || isOceanBiome(c.biomeID) ||
		(c.top != blockGrass && c.top != blockDirt && c.top != blockSnow) {
		return a, false
	}
	w := c.environment.weights
	chance := (.0005*w[regionPlains] + .012*w[regionForest] + .015*w[regionTaiga] + .001*w[regionMountain]) * (1 - smoothstep(.25, .65, c.slope))
	if w[regionTaiga] > w[regionForest]+w[regionPlains] {
		a.log, a.leaves, a.spruce = blockLogSpruce, blockLeavesSpruce, true
	}
	density := fbm2(seed+99, float32(x)*0.02, float32(z)*0.02)
	if density < -0.1 {
		return a, false
	}
	if density > 0.4 {
		chance *= 1.5
	}
	r := (hash2(seed+1, x, z) + 1) * 0.5
	if r >= chance {
		return a, false
	}
	if c.biomeID == BiomeForest && r < chance*0.05 {
		a.log, a.leaves = blockLogBirch, blockLeavesBirch
	}
	h := (hash2(seed+2, x, z) + 1) * 0.5
	a.height = 4 + int(h*3)
	if a.spruce {
		a.height = 6 + int(h*4)
	}
	return a, true
}

// Geometry is independent of loaded chunks or previously placed structures.
func (a treeAnchor) emit(put func(x, y, z int, b byte)) {
	for y := a.y; y < a.y+a.height; y++ {
		put(a.x, y, a.z, a.log)
	}
	start, end := a.y+a.height-2, a.y+a.height
	if a.spruce {
		start, end = a.y+3, a.y+a.height+1
	}
	for y := start; y <= end; y++ {
		r := 2
		if a.spruce {
			d := end - y
			r = 1
			if d > 2 && d%2 != 0 {
				r = 2
			}
			if d == 0 {
				r = 0
			}
		} else if y == end {
			r = 1
		}
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				if dx == 0 && dz == 0 && y < a.y+a.height {
					continue
				}
				if dx*dx+dz*dz > r*r+1 {
					continue
				}
				put(a.x+dx, y, a.z+dz, a.leaves)
			}
		}
	}
}

func generationIsLog(b byte) bool { return b == blockLog || b == blockLogBirch || b == blockLogSpruce }
func generationIsLeaf(b byte) bool {
	return b == blockLeaves || b == blockLeavesBirch || b == blockLeavesSpruce
}

func placeGeneratedTrees(seed uint32, cx, cz int, chunk *Chunk) {
	placeGeneratedTreesSampled(seed, cx, cz, chunk, sampleTerrainColumn)
}

func placeGeneratedTreesSampled(seed uint32, cx, cz int, chunk *Chunk, column func(uint32, int, int) terrainColumn) {
	bx, bz := cx*chunkWidth, cz*chunkWidth
	// Every intersecting anchor is replayed in global X/Z order in each chunk.
	// Logs beat leaves; the earliest anchor wins equal-priority overlaps.
	for x := bx - treeRadius; x < bx+chunkWidth+treeRadius; x++ {
		for z := bz - treeRadius; z < bz+chunkWidth+treeRadius; z++ {
			a, ok := treeAnchorFromColumn(seed, x, z, column(seed, x, z))
			if !ok {
				continue
			}
			a.emit(func(wx, y, wz int, b byte) {
				lx, lz := wx-bx, wz-bz
				if lx < 0 || lx >= chunkWidth || lz < 0 || lz >= chunkWidth || y < 1 || y >= chunkHeight {
					return
				}
				old := chunk.blocks[lx][y][lz]
				if old != blockAir && !(generationIsLog(b) && generationIsLeaf(old)) {
					return
				}
				chunk.blocks[lx][y][lz] = b
				chunk.heightMap[lx][lz] = max(chunk.heightMap[lx][lz], int16(y+1))
			})
		}
	}
}
