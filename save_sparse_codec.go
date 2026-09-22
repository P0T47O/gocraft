package main

// Emit exactly the existing palette/RLE byte order, without expanding two
// 64KiB planes and building two more 64KiB intermediate value arrays.
type chunkRunEncoder struct {
	data  []byte
	value byte
	count int
}

func (r *chunkRunEncoder) add(value byte) {
	if r.count != 0 && (value != r.value || r.count == 65535) {
		r.flush()
	}
	r.value = value
	r.count++
}

func (r *chunkRunEncoder) flush() {
	if r.count == 0 {
		return
	}
	r.data = appendUint16(r.data, uint16(r.count))
	r.data = append(r.data, r.value)
	r.count = 0
}

func encodeSparseChunk(blocks, meta *chunkPlane) ([]byte, []byte, []byte) {
	palette := make([]byte, 0, 32)
	var seen [256]bool
	var indices [256]byte
	b, m := chunkRunEncoder{}, chunkRunEncoder{}
	for x := 0; x < chunkWidth; x++ {
		for sec := 0; sec < sectionCount; sec++ {
			bs := &blocks.sections[sec]
			ms := voxelSection{}
			if meta != nil {
				ms = meta.sections[sec]
			}
			for i := x * sectionHeight * chunkWidth; i < (x+1)*sectionHeight*chunkWidth; i++ {
				id, mv := bs.uniform, ms.uniform
				if bs.data != nil {
					id = bs.data[i]
				}
				if ms.data != nil {
					mv = ms.data[i]
				}
				if !seen[id] {
					seen[id] = true
					indices[id] = byte(len(palette))
					palette = append(palette, id)
				}
				b.add(indices[id])
				m.add(mv)
			}
		}
	}
	b.flush()
	m.flush()
	return palette, b.data, m.data
}
