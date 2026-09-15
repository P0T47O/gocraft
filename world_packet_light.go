package main

import "time"

func (w *World) applyChunkLight(p *PacketChunkLight) bool {
	start := time.Now()
	defer func() { perfMon.recordLoading(phaseClientApply, start) }()
	if p.Sections == 0 || len(p.Data) != chunkLightSize(p.Sections) {
		return false
	}
	cx, cz := int(p.CX), int(p.CZ)
	c := w.getChunkIfGenerated(cx, cz)
	if c == nil {
		return false
	} // Unloaded while in transit: the next request is full.
	var changes chunkMeshChanges
	i := 0
	for sec := 0; sec < sectionCount; sec++ {
		if p.Sections&(1<<sec) != 0 {
			for x := 0; x < chunkWidth; x++ {
				for y := sec * sectionHeight; y < (sec+1)*sectionHeight; y++ {
					for z := 0; z < chunkWidth; z++ {
						v := p.Data[i]
						i++
						if c.skyLight[x][y][z] != v>>4 || c.blockLight[x][y][z] != v&15 {
							c.skyLight[x][y][z], c.blockLight[x][y][z] = v>>4, v&15
							changes.mark(x, y, z)
						}
					}
				}
			}
		}
	}
	changes.apply(w, cx, cz)
	return true
}
