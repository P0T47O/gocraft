package main

// A zero-value plane represents 16 uniform-air sections without allocating voxel
// arrays. The owner must hold the same chunk lock as for the old dense arrays.
// Do not copy a nonempty plane and mutate both copies: Clone makes ownership explicit.
type chunkPlane struct{ sections [sectionCount]voxelSection }
type voxelSection struct {
	data    *[chunkWidth * sectionHeight * chunkWidth]byte
	uniform byte
}

// CopyColumn writes a vertical slice with the snapshot's interleaved stride.
// The section pointer and representation are resolved once per 16-high span.
// mergeMax combines block light with the already-copied sky light.
func (p *chunkPlane) CopyColumn(dst []byte, index, stride, x, z, lo, hi int, mergeMax bool) {
	for y := lo; y < hi; {
		s := &p.sections[y/sectionHeight]
		end := min(hi, (y/sectionHeight+1)*sectionHeight)
		if s.data == nil {
			v := s.uniform
			if mergeMax && v == 0 {
				index += (end - y) * stride
				y = end
				continue
			}
			for ; y < end; y++ {
				if mergeMax {
					dst[index] = max(dst[index], v)
				} else {
					dst[index] = v
				}
				index += stride
			}
		} else {
			offset := (x*sectionHeight+y%sectionHeight)*chunkWidth + z
			for ; y < end; y++ {
				v := s.data[offset]
				if mergeMax {
					dst[index] = max(dst[index], v)
				} else {
					dst[index] = v
				}
				index += stride
				offset += chunkWidth
			}
		}
	}
}

func (p *chunkPlane) Fill(value byte) {
	*p = chunkPlane{}
	for i := range p.sections {
		p.sections[i].uniform = value
	}
}

func (p *chunkPlane) Get(x, y, z int) byte {
	s := &p.sections[y/sectionHeight]
	if s.data == nil {
		return s.uniform
	}
	return s.data[(x*sectionHeight+y%sectionHeight)*chunkWidth+z]
}

func (p *chunkPlane) Set(x, y, z int, value byte) {
	s := &p.sections[y/sectionHeight]
	if s.data == nil {
		if value == s.uniform {
			return
		}
		s.data = new([chunkWidth * sectionHeight * chunkWidth]byte)
		if s.uniform != 0 {
			for i := range s.data {
				s.data[i] = s.uniform
			}
		}
	}
	s.data[(x*sectionHeight+y%sectionHeight)*chunkWidth+z] = value
}

// Compact is called at bulk publication boundaries, not on every voxel write.
func (p *chunkPlane) Compact() {
	for i := range p.sections {
		s := &p.sections[i]
		if s.data == nil {
			continue
		}
		value := s.data[0]
		same := true
		for _, v := range s.data {
			if v != value {
				same = false
				break
			}
		}
		if same {
			s.uniform = value
			s.data = nil
		}
	}
}

func (p *chunkPlane) Clone() chunkPlane {
	out := *p
	for i := range out.sections {
		if d := out.sections[i].data; d != nil {
			copy := *d
			out.sections[i].data = &copy
		}
	}
	return out
}

func (p *chunkPlane) Equal(q *chunkPlane) bool {
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if p.Get(x, y, z) != q.Get(x, y, z) {
					return false
				}
			}
		}
	}
	return true
}

// Dense is a transient wire/save adapter. Persistent storage remains sparse.
func (p *chunkPlane) Dense() *[chunkWidth][chunkHeight][chunkWidth]byte {
	out := new([chunkWidth][chunkHeight][chunkWidth]byte)
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				out[x][y][z] = p.Get(x, y, z)
			}
		}
	}
	return out
}

func (p *chunkPlane) FromDense(d *[chunkWidth][chunkHeight][chunkWidth]byte) {
	*p = chunkPlane{}
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				p.Set(x, y, z, d[x][y][z])
			}
		}
	}
	p.Compact()
}

// FromWire builds canonical sections without per-voxel expansion or a second
// compaction pass. Callers validate the 65536-byte payload; nil means all zero.
func (p *chunkPlane) FromWire(data []byte, shift uint, mask byte) {
	*p = chunkPlane{}
	if data == nil {
		return
	}
	var scratch [chunkWidth * sectionHeight * chunkWidth]byte
	for sec := range p.sections {
		first := (data[sec*sectionHeight*chunkWidth] >> shift) & mask
		same := true
		for x := 0; x < chunkWidth; x++ {
			start := (x*chunkHeight + sec*sectionHeight) * chunkWidth
			for i, v := range data[start : start+sectionHeight*chunkWidth] {
				value := (v >> shift) & mask
				scratch[x*sectionHeight*chunkWidth+i] = value
				same = same && value == first
			}
		}
		if same {
			p.sections[sec].uniform = first
		} else {
			p.sections[sec].data = new([chunkWidth * sectionHeight * chunkWidth]byte)
			*p.sections[sec].data = scratch
		}
	}
}
