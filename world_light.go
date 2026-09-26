package main

func isOpaqueBlock(block byte) bool { return GetBlock(block).IsOpaque }
func lightOpaque(block, meta byte) bool {
	return isOpaqueBlock(block) || isSlab(block) && meta&shapeDouble != 0
}
func lightEmission(block byte) byte { return GetBlock(block).LightLevel }
func emitsLight(block byte) bool    { return lightEmission(block) > 0 }

type lightPos struct{ x, y, z int }

var lightDirections = [...]lightPos{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}

func (p lightPos) add(d lightPos) lightPos { return lightPos{p.x + d.x, p.y + d.y, p.z + d.z} }

func (w *World) LightAt(x, y, z int) byte {
	sky, block := w.LightSkyAt(x, y, z), w.LightBlockAt(x, y, z)
	if block > sky {
		return block
	}
	return sky
}
func (w *World) LightSkyAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	c := w.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth))
	if c == nil {
		return 15
	} // Rendering fallback only; propagation never uses it.
	return c.skyLight.Get(modFloor(x, chunkWidth), y, modFloor(z, chunkWidth))
}
func (w *World) LightBlockAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	c := w.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth))
	if c == nil {
		return 0
	}
	return c.blockLight.Get(modFloor(x, chunkWidth), y, modFloor(z, chunkWidth))
}

// initializeChunkLighting operates only on c. The generation worker calls it
// after filling blocks and before publishing c, while exclusively owning c.
// Missing neighbors supply no light. No World, locks, or mesh state is needed.
func initializeChunkLighting(c *Chunk) {
	c.skyLight.Fill(15)
	c.blockLight = chunkPlane{}
	blockQueue := make([]lightPos, 0, 64)
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			open := true
			for y := chunkHeight - 1; y >= 0; y-- {
				block := c.blocks.Get(x, y, z)
				if lightOpaque(block, c.meta.Get(x, y, z)) {
					open = false
				}
				if !open {
					c.skyLight.Set(x, y, z, 0)
				}
				if emit := lightEmission(block); emit > 0 {
					c.blockLight.Set(x, y, z, emit)
					blockQueue = append(blockQueue, lightPos{x, y, z})
				}
			}
		}
	}
	// Enqueue only dark passable cells touching direct sky, not every sunlit cell.
	skyQueue := make([]lightPos, 0, 256)
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if c.skyLight.Get(x, y, z) != 0 || lightOpaque(c.blocks.Get(x, y, z), c.meta.Get(x, y, z)) {
					continue
				}
				p := lightPos{x, y, z}
				for _, d := range lightDirections {
					n := p.add(d)
					if localLightPos(n) && c.skyLight.Get(n.x, n.y, n.z) == 15 {
						c.skyLight.Set(x, y, z, 14)
						skyQueue = append(skyQueue, p)
						break
					}
				}
			}
		}
	}
	spreadLocalLight(c, &c.skyLight, skyQueue)
	spreadLocalLight(c, &c.blockLight, blockQueue)
	c.skyLight.Compact()
	c.blockLight.Compact()
}
func localLightPos(p lightPos) bool {
	return p.x >= 0 && p.x < chunkWidth && p.y >= 0 && p.y < chunkHeight && p.z >= 0 && p.z < chunkWidth
}
func spreadLocalLight(c *Chunk, light *chunkPlane, queue []lightPos) {
	for head := 0; head < len(queue); head++ {
		p := queue[head]
		level := light.Get(p.x, p.y, p.z)
		if level <= 1 {
			continue
		} // Guard before unsigned subtraction.
		for _, d := range lightDirections {
			n := p.add(d)
			if !localLightPos(n) || lightOpaque(c.blocks.Get(n.x, n.y, n.z), c.meta.Get(n.x, n.y, n.z)) {
				continue
			}
			if light.Get(n.x, n.y, n.z) < level-1 {
				light.Set(n.x, n.y, n.z, level-1)
				queue = append(queue, n)
			}
		}
	}
}

// World lighting mutations run on the owner thread. Cache pointers (including
// missing chunks) and direct-sky columns for one transaction. No chunk locks.
type lightChunkCache struct {
	c     *Chunk
	top   [chunkWidth][chunkWidth]int
	known [chunkWidth][chunkWidth]bool
}
type lightUpdate struct {
	w      *World
	chunks map[chunkKey]*lightChunkCache
	dirty  map[sectionKey]bool
}

func newLightUpdate(w *World) *lightUpdate {
	return &lightUpdate{w: w, chunks: make(map[chunkKey]*lightChunkCache), dirty: make(map[sectionKey]bool)}
}
func (u *lightUpdate) chunk(key chunkKey) *lightChunkCache {
	if c, ok := u.chunks[key]; ok {
		return c
	}
	c := &lightChunkCache{c: u.w.getChunkIfGenerated(key.X, key.Z)}
	u.chunks[key] = c
	return c
}
func (u *lightUpdate) at(p lightPos) (*lightChunkCache, int, int) {
	if p.y < 0 || p.y >= chunkHeight {
		return nil, 0, 0
	}
	c := u.chunk(chunkKey{divFloor(p.x, chunkWidth), divFloor(p.z, chunkWidth)})
	if c.c == nil {
		return nil, 0, 0
	}
	return c, modFloor(p.x, chunkWidth), modFloor(p.z, chunkWidth)
}
func (u *lightUpdate) level(p lightPos, sky bool) byte {
	c, x, z := u.at(p)
	if c == nil {
		return 0
	}
	if sky {
		return c.c.skyLight.Get(x, p.y, z)
	}
	return c.c.blockLight.Get(x, p.y, z)
}
func (c *lightChunkCache) directSky(x, y, z int) bool {
	if !c.known[x][z] {
		c.known[x][z] = true
		c.top[x][z] = -1
		for cy := chunkHeight - 1; cy >= 0; cy-- {
			if lightOpaque(c.c.blocks.Get(x, cy, z), c.c.meta.Get(x, cy, z)) {
				c.top[x][z] = cy
				break
			}
		}
	}
	return y > c.top[x][z]
}

// Invalidate the one-voxel mesh sampling halo, including diagonal chunks and
// adjacent vertical sections at corners. Each section version advances once.
func (u *lightUpdate) mark(p lightPos) {
	if p.y < 0 || p.y >= chunkHeight {
		return
	}
	minY, maxY := p.y-1, p.y+1
	if minY < 0 {
		minY = 0
	}
	if maxY >= chunkHeight {
		maxY = chunkHeight - 1
	}
	for cx := divFloor(p.x-1, chunkWidth); cx <= divFloor(p.x+1, chunkWidth); cx++ {
		for cz := divFloor(p.z-1, chunkWidth); cz <= divFloor(p.z+1, chunkWidth); cz++ {
			key := chunkKey{cx, cz}
			if u.chunk(key).c == nil {
				continue
			}
			for sec := sectionIndexForY(minY); sec <= sectionIndexForY(maxY); sec++ {
				u.dirty[sectionKey{key.X, key.Z, sec}] = true
			}
		}
	}
}
func (u *lightUpdate) set(p lightPos, sky bool, value byte) bool {
	c, x, z := u.at(p)
	if c == nil {
		return false
	}
	light := &c.c.blockLight
	if sky {
		light = &c.c.skyLight
	}
	if light.Get(x, p.y, z) == value {
		return false
	}
	light.Set(x, p.y, z, value)
	c.c.lightDirtySections |= 1 << (p.y / sectionHeight)
	if u.w.lightChanged == nil {
		u.w.lightChanged = make(map[chunkKey]bool)
	}
	u.w.lightChanged[chunkKey{divFloor(p.x, chunkWidth), divFloor(p.z, chunkWidth)}] = true
	u.mark(p)
	return true
}
func (u *lightUpdate) flush() {
	for key := range u.dirty {
		c := u.chunk(chunkKey{key.X, key.Z}).c
		ensureChunkSections(c)
		c.invalidateMeshSection(key.Section)
	}
	if len(u.dirty) > 0 {
		u.w.dirty = true
	}
}

// Solve the local light equation until stable. Sources are explicit: emission
// for block light, unobstructed columns for sky. Every other edge loses a level,
// so removed sources cannot sustain stale cycles. Queue entries read CURRENT
// values instead of retaining obsolete or zero propagation levels.
func (u *lightUpdate) desired(p lightPos, sky bool) byte {
	c, x, z := u.at(p)
	if c == nil {
		return 0
	}
	block := c.c.blocks.Get(x, p.y, z)
	value := lightEmission(block)
	if sky {
		value = 0
	}
	if !lightOpaque(block, c.c.meta.Get(x, p.y, z)) {
		if sky && c.directSky(x, p.y, z) {
			return 15
		}
		if value < 15 {
			for _, d := range lightDirections {
				level := u.level(p.add(d), sky)
				if level > 1 && level-1 > value {
					value = level - 1
				}
			}
		}
	}
	return value
}

func (u *lightUpdate) settle(seeds []lightPos, sky bool) {
	queue := make([]lightPos, 0, 256)
	pending := make(map[lightPos]bool)
	push := func(p lightPos) {
		if p.y < 0 || p.y >= chunkHeight || pending[p] {
			return
		}
		pending[p] = true
		queue = append(queue, p)
	}
	for _, p := range seeds {
		// Unchanged borders are scanned but never enter the propagation queue.
		if u.desired(p, sky) != u.level(p, sky) {
			push(p)
		}
	}
	for head := 0; head < len(queue); head++ {
		p := queue[head]
		delete(pending, p)
		value := u.desired(p, sky)
		if u.set(p, sky, value) {
			for _, d := range lightDirections {
				push(p.add(d))
			}
		}
	}
}

// Call after publishing the locally initialized, generated chunk. Seed BOTH
// sides, since arrivals/replacements change incoming and outgoing border light.
func (w *World) stitchChunkLighting(cx, cz int) {
	u := newLightUpdate(w)
	u.stitch(cx, cz)
	u.flush()
}

func (u *lightUpdate) stitch(cx, cz int) {
	if u.chunk(chunkKey{cx, cz}).c == nil {
		return
	}
	// A locally initialized isolated chunk already has its final lighting.
	west := u.chunk(chunkKey{cx - 1, cz}).c != nil
	east := u.chunk(chunkKey{cx + 1, cz}).c != nil
	north := u.chunk(chunkKey{cx, cz - 1}).c != nil
	south := u.chunk(chunkKey{cx, cz + 1}).c != nil
	if !west && !east && !north && !south {
		return
	}
	var seeds []lightPos
	// Equal/adjacent border levels cannot change either side. Most arrivals
	// meet identical open sky or solid terrain, so avoid queuing entire walls.
	addBorder := func(a, b lightPos) {
		ac, ax, az := u.at(a)
		bc, bx, bz := u.at(b)
		if ac == nil || bc == nil {
			return
		}
		as, bs := int(ac.c.skyLight.Get(ax, a.y, az)), int(bc.c.skyLight.Get(bx, b.y, bz))
		al, bl := int(ac.c.blockLight.Get(ax, a.y, az)), int(bc.c.blockLight.Get(bx, b.y, bz))
		toA := !lightOpaque(ac.c.blocks.Get(ax, a.y, az), ac.c.meta.Get(ax, a.y, az)) && (bs > as+1 || bl > al+1)
		toB := !lightOpaque(bc.c.blocks.Get(bx, b.y, bz), bc.c.meta.Get(bx, b.y, bz)) && (as > bs+1 || al > bl+1)
		if toA || toB {
			seeds = append(seeds, a, b)
		}
	}
	x0, z0 := cx*chunkWidth, cz*chunkWidth
	for y := 0; y < chunkHeight; y++ {
		for i := 0; i < chunkWidth; i++ {
			if west {
				addBorder(lightPos{x0, y, z0 + i}, lightPos{x0 - 1, y, z0 + i})
			}
			if east {
				addBorder(lightPos{x0 + chunkWidth - 1, y, z0 + i}, lightPos{x0 + chunkWidth, y, z0 + i})
			}
			if north {
				addBorder(lightPos{x0 + i, y, z0}, lightPos{x0 + i, y, z0 - 1})
			}
			if south {
				addBorder(lightPos{x0 + i, y, z0 + chunkWidth - 1}, lightPos{x0 + i, y, z0 + chunkWidth})
			}
		}
	}
	u.settle(seeds, true)
	u.settle(seeds, false)
}
func (w *World) rebuildLightingForChunk(cx, cz int) {
	c := w.getChunkIfGenerated(cx, cz)
	if c == nil {
		return
	}
	oldSky, oldBlock := c.skyLight, c.blockLight
	initializeChunkLighting(c)
	u := newLightUpdate(w)
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if oldSky.Get(x, y, z) != c.skyLight.Get(x, y, z) || oldBlock.Get(x, y, z) != c.blockLight.Get(x, y, z) {
					if w.lightChanged == nil {
						w.lightChanged = make(map[chunkKey]bool)
					}
					w.lightChanged[chunkKey{cx, cz}] = true
					c.lightDirtySections |= 1 << (y / sectionHeight)
					u.mark(lightPos{cx*chunkWidth + x, y, cz*chunkWidth + z})
				}
			}
		}
	}
	u.stitch(cx, cz)
	u.flush()
}
func (w *World) updateBlockLight(x, y, z int, oldBlock, newBlock byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	u := newLightUpdate(w)
	p := lightPos{x, y, z}
	u.mark(p) // Geometry changes invalidate neighboring mesh samples too.
	u.settle([]lightPos{p}, false)
	u.flush()
}
func (w *World) updateSkyLight(x, y, z int) {
	if y < 0 || y >= chunkHeight {
		return
	}
	u := newLightUpdate(w)
	// An opacity edit changes direct-sky sources below it in this column.
	seeds := make([]lightPos, 0, y+1)
	for cy := y; cy >= 0; cy-- {
		seeds = append(seeds, lightPos{x, cy, z})
	}
	u.mark(lightPos{x, y, z})
	u.settle(seeds, true)
	u.flush()
}
func (w *World) setBlockLightAtInternal(x, y, z int, val byte) {
	u := newLightUpdate(w)
	u.set(lightPos{x, y, z}, false, val)
	u.flush()
}
func (w *World) setSkyLightAtInternal(x, y, z int, val byte) {
	u := newLightUpdate(w)
	u.set(lightPos{x, y, z}, true, val)
	u.flush()
}
func (w *World) setBlockLightAt(x, y, z int, val byte) { w.setBlockLightAtInternal(x, y, z, val) }
