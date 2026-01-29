package main

import (
	"gocraft/platform"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

type ChunkMesh struct {
	glMesh   *platform.Mesh
	material rl.Material
	// Cached Uniforms (initialized to -2 to verify, or we can use lazy init)
	// Actually, we can just look it up once if we store it.
	// But simpler: just execute the lookup if it's -1? No, 0 is valid.
	// Let's use -1 as "not found/not initialized" but 0 is a valid loc.
	// We'll init with -2.
	locMVP int32
	locCol int32
	init   bool
}

func (m *ChunkMesh) Draw(shader uint32, viewProj mgl32.Mat4, overrideTextureID uint32) {
	if m.glMesh == nil {
		return
	}

	platform.UseProgram(shader)

	if !m.init {
		m.locMVP = platform.GetUniformLocation(shader, "mvp")
		if m.locMVP == -1 {
			m.locMVP = platform.GetUniformLocation(shader, "matModelViewProjection")
		}
		m.locCol = platform.GetUniformLocation(shader, "colDiffuse")
		m.init = true
	}

	// Upload MVP (using pre-calculated Matrix)
	platform.UniformMatrix4fv(m.locMVP, 1, false, &viewProj[0])

	// Set colDiffuse
	if m.locCol != -1 {
		platform.Uniform4f(m.locCol, 1.0, 1.0, 1.0, 1.0)
	}

	// Bind Texture
	platform.ActiveTexture(platform.GL_TEXTURE0)
	var texID uint32
	if overrideTextureID != 0 {
		texID = overrideTextureID
	} else {
		texID = uint32(m.material.Maps.Texture.ID)
	}
	platform.BindTexture(platform.GL_TEXTURE_2D, texID)

	m.glMesh.Draw()
}

func (m *ChunkMesh) unload() {
	if m.glMesh != nil {
		m.glMesh.Unload()
		m.glMesh = nil
	}
}
