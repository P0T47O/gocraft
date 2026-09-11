package main

import "testing"

func TestMeshRenderStateResetInvalidatesBindingsAndUniforms(t *testing.T) {
	uniforms := &meshShaderState{mvp: 3, color: 4, model: 5, ready: true}
	state := meshRenderState{
		shaders: map[uint32]*meshShaderState{7: uniforms},
		shader:  7, texture: 8, shaderBound: true, textureBound: true,
	}
	state.reset()
	if state.shaderBound || state.textureBound || uniforms.ready {
		t.Fatal("external GL changes must invalidate cached state")
	}
	if state.shaders[7] != uniforms || uniforms.mvp != 3 || uniforms.color != 4 || uniforms.model != 5 {
		t.Fatal("reset must retain shader uniform locations")
	}
}
