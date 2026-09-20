package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"gocraft/platform"
	"sync"
)

var interleaveBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]platform.Vertex, 0, 4096)
	},
}

func (a *RenderAssets) applyMeshData(data map[string][]*MeshBuildData) map[string][]*ChunkMesh {
	meshes := map[string][]*ChunkMesh{}
	for path, list := range data {
		var tex rl.Texture2D
		if a.webGPU {
			// Atlas metadata is CPU-owned; do not allocate Raylib resources.
		} else if path == "atlas" && a.atlas != nil {
			tex = a.atlas.Texture
		} else {
			tex = a.loadTexture(path)
		}

		for _, d := range list {
			if d.vertCount == 0 {
				d.Reset()
				meshBuilderPool.Put(d)
				continue
			}

			// Indices are already uint32
			indices := d.indices

			buffer := interleaveBufferPool.Get().([]platform.Vertex)
			if cap(buffer) < d.vertCount {
				buffer = make([]platform.Vertex, d.vertCount)
			} else {
				buffer = buffer[:d.vertCount]
			}
			for i := 0; i < d.vertCount; i++ {
				buffer[i] = platform.Vertex{
					Position: [3]float32{d.vertices[i*3], d.vertices[i*3+1], d.vertices[i*3+2]},
					Texcoord: [2]float32{d.texcoords[i*2], d.texcoords[i*2+1]},
					Color:    [4]uint8{d.colors[i*4], d.colors[i*4+1], d.colors[i*4+2], d.colors[i*4+3]},
					Normal:   [3]float32{d.normals[i*3], d.normals[i*3+1], d.normals[i*3+2]},
				}
			}

			// Upload to PureGL (Manual Memory Management)
			// Note: UploadMesh likely copies the data, so we can reuse `buffer` immediately
			glMesh := platform.UploadMesh(buffer, indices)

			// Return buffer to pool
			interleaveBufferPool.Put(buffer)

			var material rl.Material
			if a.webGPU {
				// WebGPU binds its own atlas and sampler.
			} else if path == "atlas" && a.atlas != nil {
				material = a.getMaterial("atlas", tex)
			} else {
				material = a.getMaterial(path, tex)
			}

			meshes[path] = append(meshes[path], &ChunkMesh{
				glMesh:    glMesh,
				textureID: tex.ID,
				shaderID:  material.Shader.ID,
			})

			// Return builder to pool
			d.Reset()
			meshBuilderPool.Put(d)
		}
	}
	return meshes
}
