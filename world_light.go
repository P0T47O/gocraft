package main

func isOpaqueBlock(block byte) bool {
	return GetBlock(block).IsOpaque
}

func lightEmission(block byte) byte {
	return GetBlock(block).LightLevel
}

func emitsLight(block byte) bool {
	return lightEmission(block) > 0
}

func (w *World) LightAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return 15
	}
	sky := chunk.skyLight[lx][y][lz]
	block := chunk.blockLight[lx][y][lz]
	if block > sky {
		return block
	}
	return sky
}

func (w *World) LightSkyAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return 15
	}
	return chunk.skyLight[lx][y][lz]
}

func (w *World) LightBlockAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return 0
	}
	return chunk.blockLight[lx][y][lz]
}

func (w *World) setBlockLightAtInternal(x, y, z int, val byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return
	}
	// Caller MUST hold the lock if they are modifying this chunk OR
	// assume single-threaded access for lighting propagation.
	// For MVP stability on Client, we use a single thread for these updates.
	chunk.blockLight[lx][y][lz] = val
	ensureChunkSections(chunk)
	sec := sectionIndexForY(y)
	chunk.sectionDirty[sec] = true
	chunk.meshVersion[sec]++
}

func (w *World) setSkyLightAtInternal(x, y, z int, val byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return
	}
	chunk.skyLight[lx][y][lz] = val
	ensureChunkSections(chunk)
	sec := sectionIndexForY(y)
	chunk.sectionDirty[sec] = true
	chunk.meshVersion[sec]++
}

func (w *World) rebuildLightingForChunk(cx, cz int) {
	chunk := w.requestChunk(cx, cz)
	if !chunk.generated {
		return
	}
	chunk.mu.Lock()
	defer chunk.mu.Unlock()
	ensureChunkSections(chunk)

	// Fast Reset
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				chunk.skyLight[x][y][z] = 0
				chunk.blockLight[x][y][z] = 0
			}
		}
	}

	type node struct {
		x, y, z int
	}

	skyQueue := make([]node, 0, 1024)
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			blocked := false
			for y := chunkHeight - 1; y >= 0; y-- {
				if isOpaqueBlock(chunk.blocks[x][y][z]) {
					blocked = true
					continue
				}
				if !blocked {
					chunk.skyLight[x][y][z] = 15
					skyQueue = append(skyQueue, node{cx*chunkWidth + x, y, cz*chunkWidth + z})
				}
			}
		}
	}

	// Simple propagation (Broad World View)
	for i := 0; i < len(skyQueue); i++ {
		n := skyQueue[i]
		cur := w.LightSkyAt(n.x, n.y, n.z)
		if cur <= 1 {
			continue
		}
		next := cur - 1
		neighbors := [6]node{
			{n.x + 1, n.y, n.z}, {n.x - 1, n.y, n.z},
			{n.x, n.y + 1, n.z}, {n.x, n.y - 1, n.z},
			{n.x, n.y, n.z + 1}, {n.x, n.y, n.z - 1},
		}
		for _, nb := range neighbors {
			if nb.y < 0 || nb.y >= chunkHeight {
				continue
			}
			nx, nz := nb.x, nb.z
			ncx, ncz := divFloor(nx, chunkWidth), divFloor(nz, chunkWidth)
			nChunk := w.getChunkIfGenerated(ncx, ncz)
			if nChunk == nil {
				continue
			}
			lx, lz := modFloor(nx, chunkWidth), modFloor(nz, chunkWidth)
			if isOpaqueBlock(nChunk.blocks[lx][nb.y][lz]) {
				continue
			}
			if next > nChunk.skyLight[lx][nb.y][lz] {
				nChunk.skyLight[lx][nb.y][lz] = next
				nChunk.sectionDirty[sectionIndexForY(nb.y)] = true
				nChunk.meshVersion[sectionIndexForY(nb.y)]++
				skyQueue = append(skyQueue, nb)
			}
		}
	}

	blockQueue := make([]node, 0, 512)
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			for y := 0; y < chunkHeight; y++ {
				emit := lightEmission(chunk.blocks[x][y][z])
				if emit > 0 {
					chunk.blockLight[x][y][z] = emit
					blockQueue = append(blockQueue, node{cx*chunkWidth + x, y, cz*chunkWidth + z})
				}
			}
		}
	}

	for i := 0; i < len(blockQueue); i++ {
		n := blockQueue[i]
		cur := w.LightBlockAt(n.x, n.y, n.z)
		if cur <= 1 {
			continue
		}
		next := cur - 1
		neighbors := [6]node{
			{n.x + 1, n.y, n.z}, {n.x - 1, n.y, n.z},
			{n.x, n.y + 1, n.z}, {n.x, n.y - 1, n.z},
			{n.x, n.y, n.z + 1}, {n.x, n.y, n.z - 1},
		}
		for _, nb := range neighbors {
			if nb.y < 0 || nb.y >= chunkHeight {
				continue
			}
			nx, nz := nb.x, nb.z
			ncx, ncz := divFloor(nx, chunkWidth), divFloor(nz, chunkWidth)
			nChunk := w.getChunkIfGenerated(ncx, ncz)
			if nChunk == nil {
				continue
			}
			lx, lz := modFloor(nx, chunkWidth), modFloor(nz, chunkWidth)
			if isOpaqueBlock(nChunk.blocks[lx][nb.y][lz]) {
				continue
			}
			if next > nChunk.blockLight[lx][nb.y][lz] {
				nChunk.blockLight[lx][nb.y][lz] = next
				nChunk.sectionDirty[sectionIndexForY(nb.y)] = true
				nChunk.meshVersion[sectionIndexForY(nb.y)]++
				blockQueue = append(blockQueue, nb)
			}
		}
	}

	for sec := 0; sec < sectionCount; sec++ {
		chunk.sectionDirty[sec] = true
		chunk.meshVersion[sec]++
	}
}

func (w *World) updateBlockLight(x, y, z int, oldBlock, newBlock byte) {
	type node struct {
		x int
		y int
		z int
		l byte
	}

	decrease := make([]node, 0, 128)
	increase := make([]node, 0, 128)

	oldEmit := lightEmission(oldBlock)
	newEmit := lightEmission(newBlock)

	oldLight := w.LightBlockAt(x, y, z)
	if oldLight > 0 && (newEmit < oldEmit || !emitsLight(newBlock)) {
		w.setBlockLightAtInternal(x, y, z, 0)
		decrease = append(decrease, node{x: x, y: y, z: z, l: oldLight})
	}
	if newEmit > 0 {
		w.setBlockLightAtInternal(x, y, z, newEmit)
		increase = append(increase, node{x: x, y: y, z: z, l: newEmit})
	}

	isPassable := func(ix, iy, iz int) bool {
		if iy < 0 || iy >= chunkHeight {
			return false
		}
		return !isOpaqueBlock(w.BlockAt(ix, iy, iz))
	}

	for i := 0; i < len(decrease); i++ {
		n := decrease[i]
		neighbors := [6]node{
			{x: n.x + 1, y: n.y, z: n.z},
			{x: n.x - 1, y: n.y, z: n.z},
			{x: n.x, y: n.y + 1, z: n.z},
			{x: n.x, y: n.y - 1, z: n.z},
			{x: n.x, y: n.y, z: n.z + 1},
			{x: n.x, y: n.y, z: n.z - 1},
		}
		for _, nb := range neighbors {
			light := w.LightBlockAt(nb.x, nb.y, nb.z)
			if light == 0 {
				continue
			}
			if light < n.l {
				w.setBlockLightAtInternal(nb.x, nb.y, nb.z, 0)
				decrease = append(decrease, node{x: nb.x, y: nb.y, z: nb.z, l: light})
			} else {
				increase = append(increase, node{x: nb.x, y: nb.y, z: nb.z, l: light})
			}
		}
	}

	for i := 0; i < len(increase); i++ {
		n := increase[i]
		neighbors := [6]node{
			{x: n.x + 1, y: n.y, z: n.z},
			{x: n.x - 1, y: n.y, z: n.z},
			{x: n.x, y: n.y + 1, z: n.z},
			{x: n.x, y: n.y - 1, z: n.z},
			{x: n.x, y: n.y, z: n.z + 1},
			{x: n.x, y: n.y, z: n.z - 1},
		}
		for _, nb := range neighbors {
			if !isPassable(nb.x, nb.y, nb.z) {
				continue
			}
			target := n.l - 1
			if target <= 0 {
				continue
			}
			if w.LightBlockAt(nb.x, nb.y, nb.z) >= target {
				continue
			}
			w.setBlockLightAtInternal(nb.x, nb.y, nb.z, target)
			increase = append(increase, node{x: nb.x, y: nb.y, z: nb.z, l: target})
		}
	}
}

func (w *World) updateSkyLight(x, y, z int) {
	type node struct {
		x, y, z int
		l       byte
	}
	var increase []node
	var decrease []node

	oldLight := w.LightSkyAt(x, y, z)

	// Check if this column still sees the sky
	canSeeSky := true
	for cy := chunkHeight - 1; cy > y; cy-- {
		if isOpaqueBlock(w.BlockAt(x, cy, z)) {
			canSeeSky = false
			break
		}
	}

	newLight := byte(0)
	if canSeeSky && !isOpaqueBlock(w.BlockAt(x, y, z)) {
		newLight = 15
	} else {
		// Try to get light from neighbors
		maxNb := byte(0)
		for _, offset := range [][]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
			nb := w.LightSkyAt(x+offset[0], y+offset[1], z+offset[2])
			if nb > maxNb {
				maxNb = nb
			}
		}
		if maxNb > 1 {
			newLight = maxNb - 1
		}
	}

	if oldLight > newLight {
		w.setSkyLightAtInternal(x, y, z, newLight)
		decrease = append(decrease, node{x, y, z, oldLight})
	} else if newLight > oldLight {
		w.setSkyLightAtInternal(x, y, z, newLight)
		increase = append(increase, node{x, y, z, newLight})
	}

	// Sky Light propagation: downward is always 15 if no obstruction
	for i := 0; i < len(decrease); i++ {
		n := decrease[i]
		neighbors := [][]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
		for _, offset := range neighbors {
			nx, ny, nz := n.x+offset[0], n.y+offset[1], n.z+offset[2]
			light := w.LightSkyAt(nx, ny, nz)
			if light == 0 {
				continue
			}

			if n.l == 15 && offset[1] == -1 {
				// Special case: sunlight propagation downward
			} else if light < n.l {
				w.setSkyLightAtInternal(nx, ny, nz, 0)
				decrease = append(decrease, node{nx, ny, nz, light})
			} else if light >= n.l {
				increase = append(increase, node{nx, ny, nz, light})
			}
		}
	}

	for i := 0; i < len(increase); i++ {
		n := increase[i]
		neighbors := [][]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
		for _, offset := range neighbors {
			nx, ny, nz := n.x+offset[0], n.y+offset[1], n.z+offset[2]
			if ny < 0 || ny >= chunkHeight {
				continue
			}
			if isOpaqueBlock(w.BlockAt(nx, ny, nz)) {
				continue
			}

			target := n.l - 1
			if n.l == 15 && offset[1] == -1 {
				target = 15 // Sunlight goes straight down
			}

			if w.LightSkyAt(nx, ny, nz) < target {
				w.setSkyLightAtInternal(nx, ny, nz, target)
				increase = append(increase, node{nx, ny, nz, target})
			}
		}
	}
}

func (w *World) setBlockLightAt(x, y, z int, val byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if !chunk.generated {
		return
	}
	chunk.mu.Lock()
	chunk.blockLight[lx][y][lz] = val
	ensureChunkSections(chunk)
	sec := sectionIndexForY(y)
	chunk.sectionDirty[sec] = true
	chunk.meshVersion[sec]++
	chunk.mu.Unlock()
}
