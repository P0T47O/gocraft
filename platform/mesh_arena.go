package platform

import (
	"fmt"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"sort"
)

// Offsets are in vertices/indices, so every slice satisfies buffer alignment.
// Only the render owner allocates/frees ranges; queue writes precede new draws.
type meshRange struct{ offset, count uint32 }
type meshArenaPage struct {
	owner                 *WebGPUMeshBackend
	id                    uint64
	vertices, indices     *wgpu.Buffer
	vertexFree, indexFree []meshRange
	refs                  int
	bytes                 uint64
}

func findMeshRange(free []meshRange, count uint32) int {
	for i, r := range free {
		if r.count >= count {
			return i
		}
	}
	return -1
}
func takeMeshRange(free *[]meshRange, index int, count uint32) uint32 {
	r := &(*free)[index]
	offset := r.offset
	r.offset += count
	r.count -= count
	if r.count == 0 {
		*free = append((*free)[:index], (*free)[index+1:]...)
	}
	return offset
}
func freeMeshRange(free []meshRange, offset, count uint32) []meshRange {
	free = append(free, meshRange{offset, count})
	sort.Slice(free, func(i, j int) bool { return free[i].offset < free[j].offset })
	n := 0
	for _, r := range free {
		if n > 0 && free[n-1].offset+free[n-1].count == r.offset {
			free[n-1].count += r.count
		} else {
			free[n] = r
			n++
		}
	}
	return free[:n]
}

func (b *WebGPUMeshBackend) reserveMesh(vertices, indices uint32) (*meshArenaPage, uint32, uint32, error) {
	for _, p := range b.pages {
		vi, ii := findMeshRange(p.vertexFree, vertices), findMeshRange(p.indexFree, indices)
		if vi >= 0 && ii >= 0 {
			p.refs++
			return p, takeMeshRange(&p.vertexFree, vi, vertices), takeMeshRange(&p.indexFree, ii, indices), nil
		}
	}
	vc, ic := max(uint32(262144), vertices), max(uint32(524288), indices)
	stride := uint64(36)
	if b.compact {
		stride = 24
	}
	v, err := b.device.CreateBuffer(&wgpu.BufferDescriptor{Label: "GoCraft mesh arena vertices", Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst, Size: uint64(vc) * stride})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("create vertex arena: %w", err)
	}
	i, err := b.device.CreateBuffer(&wgpu.BufferDescriptor{Label: "GoCraft mesh arena indices", Usage: gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst, Size: uint64(ic) * 4})
	if err != nil {
		v.Release()
		return nil, 0, 0, fmt.Errorf("create index arena: %w", err)
	}
	b.nextPage++
	p := &meshArenaPage{owner: b, id: b.nextPage, vertices: v, indices: i, vertexFree: []meshRange{{0, vc}}, indexFree: []meshRange{{0, ic}}, bytes: uint64(vc)*stride + uint64(ic)*4, refs: 1}
	b.pages = append(b.pages, p)
	MeshBufferBytes.Add(int64(p.bytes))
	return p, takeMeshRange(&p.vertexFree, 0, vertices), takeMeshRange(&p.indexFree, 0, indices), nil
}

func (p *meshArenaPage) release(vo, vc, io, ic uint32) {
	p.vertexFree = freeMeshRange(p.vertexFree, vo, vc)
	p.indexFree = freeMeshRange(p.indexFree, io, ic)
	p.refs--
	if p.refs != 0 {
		return
	}
	if p.vertices != nil {
		p.vertices.Release()
	}
	if p.indices != nil {
		p.indices.Release()
	}
	MeshBufferBytes.Add(-int64(p.bytes))
	for i, page := range p.owner.pages {
		if page == p {
			p.owner.pages = append(p.owner.pages[:i], p.owner.pages[i+1:]...)
			break
		}
	}
}

// Opaque/cutout draws may be grouped by arena. Transparent draws retain their
// existing depth order and use DrawPass instead. Counts remain actual draws.
func (b *WebGPUMeshBackend) DrawGrouped(pass *wgpu.RenderPassEncoder, meshes []MeshHandle) (draws, triangles int, err error) {
	b.drawItems = b.drawItems[:0]
	defer func() { clear(b.drawItems); b.drawItems = b.drawItems[:0] }()
	for _, mesh := range meshes {
		if mesh == nil {
			continue
		}
		m, ok := mesh.(*webGPUMesh)
		if !ok {
			return 0, 0, fmt.Errorf("foreign mesh in draw group")
		}
		if m.uploadErr != nil {
			return 0, 0, m.uploadErr
		}
		if m.indexCount > 0 && m.page != nil {
			b.drawItems = append(b.drawItems, m)
		}
	}
	sort.SliceStable(b.drawItems, func(i, j int) bool { return b.drawItems[i].page.id < b.drawItems[j].page.id })
	var bound *meshArenaPage
	for _, m := range b.drawItems {
		if bound != m.page {
			bound = m.page
			pass.SetVertexBuffer(0, bound.vertices, 0)
			pass.SetIndexBuffer(bound.indices, gputypes.IndexFormatUint32, 0)
		}
		pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: uint32(m.indexCount), InstanceCount: 1, FirstIndex: m.firstIndex, BaseVertex: int32(m.firstVertex)})
		draws++
		triangles += int(m.indexCount) / 3
	}
	return draws, triangles, nil
}
