package platform

import (
	"fmt"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// WebGPUMeshBackend owns WebGPU mesh buffers for the renderer experiment.
// It deliberately does not own the surface/pipeline: those belong to the
// higher-level renderer because they are frame- and material-specific.
type WebGPUMeshBackend struct {
	device *wgpu.Device
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

// UploadChecked creates vertex/index buffers from the backend-neutral GoCraft
// mesh payload. Buffer creation and upload remain on the render thread.
func (b *WebGPUMeshBackend) UploadChecked(vertices []Vertex, indices []uint32) (MeshHandle, error) {
	if b == nil || b.device == nil {
		return nil, fmt.Errorf("webgpu mesh backend has no device")
	}
	if len(vertices) == 0 || len(indices) == 0 {
		return &webGPUMesh{indexCount: int32(len(indices))}, nil
	}

	vertexBytes := uint64(len(vertices)) * uint64(unsafe.Sizeof(Vertex{}))
	indexBytes := uint64(len(indices)) * uint64(unsafe.Sizeof(indices[0]))

	vertexBuffer, err := b.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft chunk vertices",
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		Size:  vertexBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("create WebGPU vertex buffer: %w", err)
	}
	if vertexBuffer == nil {
		return nil, fmt.Errorf("create WebGPU vertex buffer: nil buffer")
	}

	indexBuffer, err := b.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft chunk indices",
		Usage: gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		Size:  indexBytes,
	})
	if err != nil {
		vertexBuffer.Release()
		return nil, fmt.Errorf("create WebGPU index buffer: %w", err)
	}
	if indexBuffer == nil {
		vertexBuffer.Release()
		return nil, fmt.Errorf("create WebGPU index buffer: nil buffer")
	}

	queue := b.device.Queue()
	if queue == nil {
		indexBuffer.Release()
		vertexBuffer.Release()
		return nil, fmt.Errorf("webgpu device returned nil queue")
	}

	vertexSrc := unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), int(vertexBytes))
	if err := queue.WriteBuffer(vertexBuffer, 0, vertexSrc); err != nil {
		indexBuffer.Release()
		vertexBuffer.Release()
		return nil, fmt.Errorf("upload WebGPU vertex buffer: %w", err)
	}
	indexSrc := unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), int(indexBytes))
	if err := queue.WriteBuffer(indexBuffer, 0, indexSrc); err != nil {
		indexBuffer.Release()
		vertexBuffer.Release()
		return nil, fmt.Errorf("upload WebGPU index buffer: %w", err)
	}

	return &webGPUMesh{
		vertexBuffer: vertexBuffer,
		indexBuffer:  indexBuffer,
		vertexBytes:  vertexBytes,
		indexBytes:   indexBytes,
		indexCount:   int32(len(indices)),
	}, nil
}

func (b *WebGPUMeshBackend) Draw(mesh MeshHandle) {
	// The generic renderer seam does not yet carry a WebGPU render pass. World
	// integration will route this through DrawPass once the frame renderer owns
	// the WebGPU surface/pipeline. Keeping this a no-op prevents accidental GL use.
}

// DrawPass binds a WebGPU mesh to an existing render pass and emits one indexed
// draw. This is the bridge used by the smoke probe and, later, the world pass.
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
		FirstIndex:    0,
		BaseVertex:    0,
		FirstInstance: 0,
	})
	return nil
}

type webGPUMesh struct {
	vertexBuffer *wgpu.Buffer
	indexBuffer  *wgpu.Buffer
	vertexBytes  uint64
	indexBytes   uint64
	indexCount   int32
	uploadErr    error
}

func (m *webGPUMesh) Draw() {}

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
	if m.indexBuffer != nil {
		m.indexBuffer.Release()
		m.indexBuffer = nil
	}
	if m.vertexBuffer != nil {
		m.vertexBuffer.Release()
		m.vertexBuffer = nil
	}
	m.vertexBytes = 0
	m.indexBytes = 0
	m.indexCount = 0
}

// EnableExperimentalWebGPU switches future mesh uploads to WebGPU. The caller
// must create/own the device and ensure all old backend meshes are unloaded
// before switching.
func EnableExperimentalWebGPU(device *wgpu.Device) *WebGPUMeshBackend {
	backend := NewWebGPUMeshBackend(device)
	SetMeshBackend(backend)
	return backend
}
