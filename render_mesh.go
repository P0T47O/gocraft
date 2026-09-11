package main

import (
	"gocraft/platform"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

type ChunkMesh struct {
	glMesh   *platform.Mesh
	material rl.Material
}

type meshShaderState struct {
	mvp, color, model int32
	ready             bool
}

// meshRenderState is owned by the render thread. reset must be called at frame
// boundaries and after any Raylib/external GL drawing; uniforms are valid only
// for the shared view-projection matrix between resets.
type meshRenderState struct {
	shaders                   map[uint32]*meshShaderState
	shader, texture           uint32
	shaderBound, textureBound bool
}

func (s *meshRenderState) reset() {
	s.shaderBound = false
	s.textureBound = false
	for _, shader := range s.shaders {
		shader.ready = false
	}
}

func (s *meshRenderState) draw(m *ChunkMesh, shader uint32, viewProj mgl32.Mat4, overrideTextureID uint32) {
	if m == nil || m.glMesh == nil {
		return
	}
	if !s.shaderBound || s.shader != shader {
		platform.UseProgram(shader)
		s.shader, s.shaderBound = shader, true
	}
	uniforms := s.shaders[shader]
	if uniforms == nil {
		if s.shaders == nil {
			s.shaders = make(map[uint32]*meshShaderState)
		}
		uniforms = &meshShaderState{
			mvp:   platform.GetUniformLocation(shader, "mvp"),
			color: platform.GetUniformLocation(shader, "colDiffuse"),
			model: platform.GetUniformLocation(shader, "matModel"),
		}
		if uniforms.mvp == -1 {
			uniforms.mvp = platform.GetUniformLocation(shader, "matModelViewProjection")
		}
		s.shaders[shader] = uniforms
	}
	if !uniforms.ready {
		platform.UniformMatrix4fv(uniforms.mvp, 1, false, &viewProj[0])
		// Chunk vertices are world-space. Raylib torch models can overwrite
		// matModel on the same shader, so restore identity after each reset.
		if uniforms.model != -1 {
			identity := mgl32.Ident4()
			platform.UniformMatrix4fv(uniforms.model, 1, false, &identity[0])
		}
		if uniforms.color != -1 {
			platform.Uniform4f(uniforms.color, 1, 1, 1, 1)
		}
		uniforms.ready = true
	}
	texture := overrideTextureID
	if texture == 0 {
		texture = m.material.Maps.Texture.ID
	}
	if !s.textureBound {
		platform.ActiveTexture(platform.GL_TEXTURE0)
	}
	if !s.textureBound || s.texture != texture {
		platform.BindTexture(platform.GL_TEXTURE_2D, texture)
		s.texture, s.textureBound = texture, true
	}
	m.glMesh.Draw()
}

// Draw is a standalone draw with no assumptions about caller-owned GL state.
// World.Draw shares one meshRenderState across its meshes instead.
func (m *ChunkMesh) Draw(shader uint32, viewProj mgl32.Mat4, overrideTextureID uint32) {
	var state meshRenderState
	state.draw(m, shader, viewProj, overrideTextureID)
}

func (m *ChunkMesh) unload() {
	if m.glMesh != nil {
		m.glMesh.Unload()
		m.glMesh = nil
	}
}
