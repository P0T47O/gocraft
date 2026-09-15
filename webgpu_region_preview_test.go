//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

// TestWebGPURegionPreview renders all nine chunks used by the mesher's neighbor
// lookups. The single-chunk preview intentionally culls center-chunk boundary
// faces against its eight neighbors, so drawing only the center exposes caves
// and section interiors through those missing neighbor chunks. Rendering the
// complete 3x3 region makes that culling context visually coherent.
func TestWebGPURegionPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGION_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_REGION_PREVIEW=1 to run the interactive 3x3 WebGPU region preview")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initBlockRegistry()
	const seed uint32 = 12345

	chunks := make(map[chunkKey]*Chunk, 9)
	maxTop := 1
	for cx := -1; cx <= 1; cx++ {
		for cz := -1; cz <= 1; cz++ {
			chunk := &Chunk{}
			generateChunkData(seed, cx, cz, chunk)
			chunk.rebuildHeightMap()
			initializeChunkLighting(chunk)
			for x := 0; x < chunkWidth; x++ {
				for z := 0; z < chunkWidth; z++ {
					if int(chunk.heightMap[x][z]) > maxTop {
						maxTop = int(chunk.heightMap[x][z])
					}
				}
			}
			chunks[chunkKey{X: cx, Z: cz}] = chunk
		}
	}

	getChunk := func(wx, wz int) (*Chunk, int, int) {
		cx := divFloor(wx, chunkWidth)
		cz := divFloor(wz, chunkWidth)
		chunk := chunks[chunkKey{X: cx, Z: cz}]
		if chunk == nil {
			return nil, 0, 0
		}
		return chunk, modFloor(wx, chunkWidth), modFloor(wz, chunkWidth)
	}
	getBlock := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return blockAir
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return blockAir
		}
		return chunk.blocks[lx][wy][lz]
	}
	getLight := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return 15
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return 15
		}
		sky := chunk.skyLight[lx][wy][lz]
		block := chunk.blockLight[lx][wy][lz]
		if block > sky {
			return block
		}
		return sky
	}
	getMeta := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return 0
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return 0
		}
		return chunk.meta[lx][wy][lz]
	}

	assets := &RenderAssets{}
	type cpuRegionMesh struct {
		cx, cz  int
		results map[string]map[string][]*MeshBuildData
	}
	cpuMeshes := make([]cpuRegionMesh, 0, 9)
	for cx := -1; cx <= 1; cx++ {
		for cz := -1; cz <= 1; cz++ {
			chunk := chunks[chunkKey{X: cx, Z: cz}]
			results := assets.buildAllMeshData(
				&chunk.heightMap,
				cx*chunkWidth,
				cz*chunkWidth,
				0,
				chunkHeight,
				getBlock,
				getLight,
				getMeta,
				seed,
			)
			cpuMeshes = append(cpuMeshes, cpuRegionMesh{cx: cx, cz: cz, results: results})
		}
	}
	defer func() {
		for _, item := range cpuMeshes {
			releaseMeshResults(item.results)
		}
	}()

	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(1200, 760, "GoCraft 3x3 real region - WebGPU preview")
	defer rl.CloseWindow()
	rl.SetExitKey(0)

	hwnd := uintptr(rl.GetWindowHandle())
	if hwnd == 0 {
		t.Fatal("raylib returned a null native window handle")
	}

	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		t.Fatalf("create WebGPU instance: %v", err)
	}
	defer instance.Release()
	surface, err := instance.CreateSurfaceUnsafe(wgpu.SurfaceTargetFromWindowsHWND(0, hwnd))
	if err != nil {
		t.Fatalf("create WebGPU surface: %v", err)
	}
	defer surface.Release()
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surface,
	})
	if err != nil {
		t.Fatalf("request WebGPU adapter: %v", err)
	}
	defer adapter.Release()
	info := adapter.Info()
	t.Logf("WebGPU adapter: %s, backend=%v, deviceType=%v", info.Name, info.Backend, info.DeviceType)
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("request WebGPU device: %v", err)
	}
	defer device.Release()
	queue := device.Queue()
	if queue == nil {
		t.Fatal("WebGPU device returned nil queue")
	}

	format := gputypes.TextureFormatBGRA8Unorm
	caps := adapter.GetSurfaceCapabilities(surface)
	if caps != nil && len(caps.Formats) > 0 {
		format = caps.Formats[0]
		for _, candidate := range caps.Formats {
			if candidate == gputypes.TextureFormatBGRA8Unorm {
				format = candidate
				break
			}
		}
	}

	pipeline, cameraBindGroup, cameraBuffer, cleanupPipeline, err := createWebGPUChunkPreviewPipeline(device, format)
	if err != nil {
		t.Fatalf("create region preview pipeline: %v", err)
	}
	defer cleanupPipeline()

	backend := platform.NewWebGPUMeshBackend(device)
	meshes := make([]platform.MeshHandle, 0, 64)
	verticesTotal, indicesTotal := 0, 0
	for _, item := range cpuMeshes {
		for _, passName := range []string{"opaque", "cutout"} {
			for _, list := range item.results[passName] {
				for _, data := range list {
					if data == nil || data.vertCount == 0 || len(data.indices) == 0 {
						continue
					}
					vertices := make([]platform.Vertex, data.vertCount)
					for i := 0; i < data.vertCount; i++ {
						vertices[i] = platform.Vertex{
							Position: [3]float32{data.vertices[i*3], data.vertices[i*3+1], data.vertices[i*3+2]},
							Texcoord: [2]float32{data.texcoords[i*2], data.texcoords[i*2+1]},
							Color:    [4]uint8{data.colors[i*4], data.colors[i*4+1], data.colors[i*4+2], data.colors[i*4+3]},
							Normal:   [3]float32{data.normals[i*3], data.normals[i*3+1], data.normals[i*3+2]},
						}
					}
					mesh, err := backend.UploadChecked(vertices, data.indices)
					if err != nil {
						t.Fatalf("upload region chunk (%d,%d) %s mesh: %v", item.cx, item.cz, passName, err)
					}
					meshes = append(meshes, mesh)
					verticesTotal += len(vertices)
					indicesTotal += len(data.indices)
				}
			}
		}
	}
	defer func() {
		for _, mesh := range meshes {
			mesh.Unload()
		}
	}()
	if len(meshes) == 0 {
		t.Fatal("3x3 region mesher produced no opaque/cutout meshes")
	}

	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	var depthTexture *wgpu.Texture
	var depthView *wgpu.TextureView
	reconfigure := func() error {
		if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
			Format: format, Usage: gputypes.TextureUsageRenderAttachment,
			Width: width, Height: height,
			AlphaMode: gputypes.CompositeAlphaModeOpaque,
			PresentMode: gputypes.PresentModeFifo,
		}); err != nil {
			return err
		}
		if depthView != nil {
			depthView.Release()
			depthView = nil
		}
		if depthTexture != nil {
			depthTexture.Release()
			depthTexture = nil
		}
		depthTexture, depthView, err = createWebGPUChunkPreviewDepth(device, width, height)
		return err
	}
	if err := reconfigure(); err != nil {
		t.Fatalf("configure region preview: %v", err)
	}
	defer func() {
		if depthView != nil {
			depthView.Release()
		}
		if depthTexture != nil {
			depthTexture.Release()
		}
	}()

	t.Logf("3x3 real region ready: chunks=9 meshes=%d vertices=%d indices=%d triangles=%d maxTop=%d", len(meshes), verticesTotal, indicesTotal, indicesTotal/3, maxTop)
	t.Log("Expected result: one coherent 3x3 generated terrain slab; unlike the single-center preview, neighbor-culling no longer leaves eight invisible chunks around the rendered geometry.")

	started := time.Now()
	for {
		rl.PollInputEvents()
		if rl.WindowShouldClose() {
			break
		}
		newWidth := uint32(max(1, rl.GetScreenWidth()))
		newHeight := uint32(max(1, rl.GetScreenHeight()))
		if newWidth != width || newHeight != height {
			width, height = newWidth, newHeight
			if err := reconfigure(); err != nil {
				t.Fatalf("resize region preview: %v", err)
			}
		}
		angle := float32(time.Since(started).Seconds()) * 0.10
		if err := updateWebGPURegionPreviewCamera(queue, cameraBuffer, width, height, maxTop, angle); err != nil {
			t.Fatalf("update region camera: %v", err)
		}
		if err := drawWebGPUChunkPreviewFrame(device, queue, surface, pipeline, cameraBindGroup, depthView, backend, meshes); err != nil {
			t.Fatalf("draw region preview: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func updateWebGPURegionPreviewCamera(queue *wgpu.Queue, buffer *wgpu.Buffer, width, height uint32, maxTop int, angle float32) error {
	aspect := float32(width) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(55), aspect, 0.1, 1000.0)

	// Chunks -1..1 span [-chunkWidth, 2*chunkWidth), whose center is half a
	// chunk in world coordinates. Aim below the surface so the outer walls also
	// make the vertical scale obvious without centering on the bedrock layer.
	center := mgl32.Vec3{float32(chunkWidth) * 0.5, float32(maxTop) * 0.62, float32(chunkWidth) * 0.5}
	radius := float32(chunkWidth) * 5.2
	eye := mgl32.Vec3{
		center.X() + float32(math.Cos(float64(angle)))*radius,
		center.Y() + float32(maxTop)*0.42,
		center.Z() + float32(math.Sin(float64(angle)))*radius,
	}
	view := mgl32.LookAtV(eye, center, mgl32.Vec3{0, 1, 0})
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0.5, 0,
		0, 0, 0.5, 1,
	}
	vp := clipCorrection.Mul4(projection).Mul4(view)
	bytes := make([]byte, webGPUChunkPreviewCameraBytes)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(vp[i]))
	}
	if err := queue.WriteBuffer(buffer, 0, bytes); err != nil {
		return fmt.Errorf("write region camera uniform: %w", err)
	}
	return nil
}
