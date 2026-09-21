//go:build windows

package main

import (
	"encoding/binary"
	"github.com/gogpu/wgpu"
	"gocraft/platform"
	"image/color"
	"math"
	"os"
	"testing"
)

// Render flat and interpolated lighting side-by-side through a WebGPU vertex
// pipeline, then assert a constant reference and an interpolated pixel ramp.
func TestSmoothLightingPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_LIGHT_PREVIEW") != "1" {
		t.Skip("opt-in GPU shading preview")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	pipeline, group, cameraBuffer, cleanup, err := createWebGPUChunkPreviewPipeline(r.device, r.format)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	identity := make([]byte, 64)
	for _, i := range []int{0, 5, 10, 15} {
		binary.LittleEndian.PutUint32(identity[i*4:], math.Float32bits(1))
	}
	if err := r.queue.WriteBuffer(cameraBuffer, 0, identity); err != nil {
		t.Fatal(err)
	}
	backend := platform.NewWebGPUMeshBackend(r.device)
	light := func(x, y, z int) byte { return byte(max(0, 15-int(math.Abs(float64(x-7))+math.Abs(float64(z-7))))) }
	blocks := func(x, y, z int) byte {
		if y == 0 {
			return blockStone
		}
		return blockAir
	}
	a := &RenderAssets{}
	var meshes []platform.MeshHandle
	for panel := 0; panel < 2; panel++ {
		mb := new(meshBuilder)
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				ao, levels := sampleFaceLighting(x, 0, z, lightTop, blocks, light)
				if panel == 0 {
					v := float32(light(x, 1, z))
					levels = [4]float32{v, v, v, v}
				}
				colors := a.applyAOSmooth(blockStone, meshColor(255, 255, 255, 255), ao, levels, make([]color.RGBA, 4))
				var positions [12]float32
				for i, c := range faceLightLayouts[lightTop].corners {
					positions[i*3] = -1 + float32(panel) + (float32(x)+.5+float32(c[0])*.5)/16
					positions[i*3+1] = 1 - (float32(z)+.5+float32(c[1])*.5)/8
				}
				mb.addFaceSmooth(positions[:], meshVec3(0, 0, 1), []float32{0, 0, 1, 0, 1, 1, 0, 1}, colors)
			}
		}
		vertices := make([]platform.Vertex, mb.vertCount)
		for i := range vertices {
			copy(vertices[i].Position[:], mb.vertices[i*3:i*3+3])
			copy(vertices[i].Color[:], mb.colors[i*4:i*4+4])
			copy(vertices[i].Texcoord[:], mb.texcoords[i*2:i*2+2])
		}
		for i := range vertices {
			vertices[i].Normal = [3]float32{0, 0, 1}
		}
		mesh, err := backend.UploadChecked(vertices, mb.indices)
		if err != nil {
			t.Fatal(err)
		}
		meshes = append(meshes, mesh)
	}

	defer func() {
		for _, mesh := range meshes {
			mesh.Unload()
		}
	}()
	image := captureNativePreview(t, r, 1024, 512, func(pass *wgpu.RenderPassEncoder) error {
		pass.SetPipeline(pipeline)
		pass.SetBindGroup(0, group, nil)
		for _, mesh := range meshes {
			if err := backend.DrawPass(pass, mesh); err != nil {
				return err
			}
		}
		return nil
	})
	if image.RGBAAt(161, 240).R != image.RGBAAt(186, 240).R {
		t.Fatal("flat reference not flat")
	}
	if image.RGBAAt(673, 240).R == image.RGBAAt(698, 240).R {
		t.Fatal("GPU did not interpolate smooth light")
	}
	saveNativePreview(t, image, "work/smooth-light-comparison.png")
}
