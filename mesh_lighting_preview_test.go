package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
	"gocraft/platform"
	"math"
	"os"
	"runtime"
	"testing"
)

// Controlled A/B using the production vertex layout, quad triangulation and
// shader. Synthetic light field isolates shading from texture noise and fog.
func TestSmoothLightingPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_LIGHT_PREVIEW") != "1" {
		t.Skip("opt-in GPU shading preview")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1024, 512, "Smooth light comparison")
	defer rl.CloseWindow()
	initBlockRegistry()
	shader := loadCutoutShader()
	defer rl.UnloadShader(shader)
	img := rl.GenImageColor(1, 1, rl.White)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	defer rl.UnloadTexture(tex)
	target := rl.LoadRenderTexture(1024, 512)
	defer rl.UnloadRenderTexture(target)
	light := func(x, y, z int) byte { return byte(max(0, 15-int(math.Abs(float64(x-7))+math.Abs(float64(z-7))))) }
	blocks := func(x, y, z int) byte {
		if y == 0 {
			return blockStone
		}
		return blockAir
	}
	a := &RenderAssets{}
	rl.BeginTextureMode(target)
	rl.ClearBackground(rl.Black)
	rl.DrawRenderBatchActive()
	rl.DisableBackfaceCulling()
	for panel := 0; panel < 2; panel++ {
		mb := new(meshBuilder)
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				ao, levels := sampleFaceLighting(x, 0, z, lightTop, blocks, light)
				if panel == 0 {
					v := float32(light(x, 1, z))
					levels = [4]float32{v, v, v, v}
				}
				colors := a.applyAOSmooth(blockStone, rl.White, ao, levels, make([]rl.Color, 4))
				var positions [12]float32
				for i, c := range faceLightLayouts[lightTop].corners {
					positions[i*3] = -1 + float32(panel) + (float32(x)+.5+float32(c[0])*.5)/16
					positions[i*3+1] = 1 - (float32(z)+.5+float32(c[1])*.5)/8
				}
				mb.addFaceSmooth(positions[:], rl.NewVector3(0, 0, 1), []float32{0, 0, 1, 0, 1, 1, 0, 1}, colors)
			}
		}
		vertices := make([]platform.Vertex, mb.vertCount)
		for i := range vertices {
			copy(vertices[i].Position[:], mb.vertices[i*3:i*3+3])
			copy(vertices[i].Color[:], mb.colors[i*4:i*4+4])
			copy(vertices[i].Texcoord[:], mb.texcoords[i*2:i*2+2])
		}
		mesh := &ChunkMesh{glMesh: platform.UploadMesh(vertices, mb.indices)}
		mesh.Draw(shader.ID, mgl32.Ident4(), tex.ID)
		mesh.unload()
	}
	platform.UseProgram(0)
	rl.EnableBackfaceCulling()
	rl.EndTextureMode()
	result := rl.LoadImageFromTexture(target.Texture)
	defer rl.UnloadImage(result)
	rl.ImageFlipVertical(result)
	colors := rl.LoadImageColors(result)
	defer rl.UnloadImageColors(colors)
	// Within one tile: old left panel is flat; right panel has a smooth ramp.
	if colors[240*1024+161].R != colors[240*1024+186].R {
		t.Fatal("flat reference not flat")
	}
	if colors[240*1024+673].R == colors[240*1024+698].R {
		t.Fatal("GPU did not interpolate smooth light")
	}
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	if !rl.ExportImage(*result, "work/smooth-light-comparison.png") {
		t.Fatal("preview export failed")
	}
}
