package platform

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// WebGPUMeshBackend owns WebGPU mesh buffers for the world renderer.
// It deliberately does not own the surface/pipeline: those belong to the
// higher-level renderer because they are frame- and material-specific.
type WebGPUMeshBackend struct {
	device    *wgpu.Device
	compact   bool
	packing   []CompactVertex
	pages     []*meshArenaPage
	nextPage  uint64
	drawItems []*webGPUMesh
}

func NewWebGPUMeshBackend(device *wgpu.Device) *WebGPUMeshBackend {
	return &WebGPUMeshBackend{device: device}
}

func (b *WebGPUMeshBackend) Upload(vertices []Vertex, indices []uint32) MeshHandle {
	mesh, err := b.UploadChecked(vertices, indices)
	if err != nil {
		return &webGPUMesh{uploadErr: err, indexCount: int32(len(indices))}
	}
	return mesh
}

// UploadChecked reserves shared vertex/index ranges for a GoCraft mesh.
// Buffer allocation, uploads and range reclamation stay on the render thread.
func (b *WebGPUMeshBackend) UploadChecked(vertices []Vertex, indices []uint32) (MeshHandle, error) {
	if b == nil || b.device == nil {
		return nil, fmt.Errorf("webgpu mesh backend has no device")
	}
	if len(vertices) == 0 || len(indices) == 0 {
		return &webGPUMesh{indexCount: int32(len(indices))}, nil
	}

	vertexBytes := uint64(len(vertices)) * uint64(unsafe.Sizeof(Vertex{}))
	if b.compact {
		vertexBytes = uint64(len(vertices)) * uint64(unsafe.Sizeof(CompactVertex{}))
	}
	indexBytes := uint64(len(indices)) * uint64(unsafe.Sizeof(indices[0]))

	queue := b.device.Queue()
	if queue == nil {
		return nil, fmt.Errorf("webgpu device returned nil queue")
	}
	page, vo, io, err := b.reserveMesh(uint32(len(vertices)), uint32(len(indices)))
	if err != nil {
		return nil, err
	}
	stride := vertexBytes / uint64(len(vertices))

	vertexSrc := unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), int(vertexBytes))
	if b.compact {
		b.packing = PackCompactVertices(b.packing, vertices)
		vertexSrc = unsafe.Slice((*byte)(unsafe.Pointer(&b.packing[0])), int(vertexBytes))
	}
	if err := queue.WriteBuffer(page.vertices, uint64(vo)*stride, vertexSrc); err != nil {
		page.release(vo, uint32(len(vertices)), io, uint32(len(indices)))
		return nil, fmt.Errorf("upload WebGPU vertex buffer: %w", err)
	}
	indexSrc := unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), int(indexBytes))
	if err := queue.WriteBuffer(page.indices, uint64(io)*4, indexSrc); err != nil {
		page.release(vo, uint32(len(vertices)), io, uint32(len(indices)))
		return nil, fmt.Errorf("upload WebGPU index buffer: %w", err)
	}

	atomic.AddInt64(&ActiveMeshCount, 1)
	MeshUploadedBytes.Add(vertexBytes + indexBytes)
	return &webGPUMesh{
		counted:      true,
		vertexBuffer: page.vertices,
		indexBuffer:  page.indices,
		page:         page, firstVertex: vo, firstIndex: io, vertexCount: uint32(len(vertices)),
		vertexBytes: vertexBytes,
		indexBytes:  indexBytes,
		indexCount:  int32(len(indices)),
	}, nil
}

// DrawPass binds a WebGPU mesh to an existing render pass and emits one indexed
// draw. Used by the production world renderer and native previews.
func (b *WebGPUMeshBackend) DrawPass(pass *wgpu.RenderPassEncoder, mesh MeshHandle) error {
	if pass == nil || mesh == nil {
		return nil
	}
	gpu, ok := mesh.(*webGPUMesh)
	if !ok {
		return fmt.Errorf("mesh is not owned by WebGPU backend")
	}
	if gpu.uploadErr != nil {
		return gpu.uploadErr
	}
	if gpu.indexCount == 0 || gpu.vertexBuffer == nil || gpu.indexBuffer == nil {
		return nil
	}
	pass.SetVertexBuffer(0, gpu.vertexBuffer, 0)
	pass.SetIndexBuffer(gpu.indexBuffer, gputypes.IndexFormatUint32, 0)
	pass.DrawIndexed(gputypes.DrawIndexedArgs{
		IndexCount:    uint32(gpu.indexCount),
		InstanceCount: 1,
		FirstIndex:    gpu.firstIndex,
		BaseVertex:    int32(gpu.firstVertex),
		FirstInstance: 0,
	})
	return nil
}

type webGPUMesh struct {
	page                                 *meshArenaPage
	firstVertex, firstIndex, vertexCount uint32
	counted                              bool
	vertexBuffer                         *wgpu.Buffer
	indexBuffer                          *wgpu.Buffer
	vertexBytes                          uint64
	indexBytes                           uint64
	indexCount                           int32
	uploadErr                            error
}

func (m *webGPUMesh) IndexCount() int32 {
	if m == nil {
		return 0
	}
	return m.indexCount
}

func (m *webGPUMesh) Unload() {
	if m == nil {
		return
	}
	if m.counted {
		atomic.AddInt64(&ActiveMeshCount, -1)
		m.counted = false
	}
	if m.page != nil {
		m.page.release(m.firstVertex, m.vertexCount, m.firstIndex, uint32(m.indexCount))
		m.page = nil
	}
	m.indexBuffer, m.vertexBuffer = nil, nil
	m.vertexBytes = 0
	m.indexBytes = 0
	m.indexCount = 0
}

// EnableExperimentalWebGPU switches future mesh uploads to WebGPU. The caller
// must create/own the device and ensure all old backend meshes are unloaded
// before switching.
func EnableExperimentalWebGPU(device *wgpu.Device) *WebGPUMeshBackend {
	backend := NewWebGPUMeshBackend(device)
	backend.compact = true
	SetMeshBackend(backend)
	return backend
}
