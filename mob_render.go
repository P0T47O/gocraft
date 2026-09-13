package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"unsafe"
)

type MobRenderer struct {
	Content  *MobContent
	Models   map[string][]rl.Model
	Textures map[string]rl.Texture2D
}

func newMobRenderer(content *MobContent) (*MobRenderer, error) {
	r := &MobRenderer{Content: content, Models: map[string][]rl.Model{}, Textures: map[string]rl.Texture2D{}}
	for id, model := range content.Models {
		tex := rl.LoadTexture(model.Texture)
		if tex.ID == 0 {
			r.Close()
			return nil, fmt.Errorf("model %s: cannot load texture %s", id, model.Texture)
		}
		r.Textures[id] = tex
		rl.SetTextureFilter(tex, rl.FilterPoint)
		if float32(tex.Width) != model.TextureSize[0] || float32(tex.Height) != model.TextureSize[1] {
			r.Close()
			return nil, fmt.Errorf("model %s: texture size mismatch %dx%d", id, tex.Width, tex.Height)
		}
		for _, b := range model.Bones {
			mesh := rl.GenMeshCube(b.Size[0], b.Size[1], b.Size[2])
			v := unsafe.Slice(mesh.Vertices, int(mesh.VertexCount)*3)
			n := unsafe.Slice(mesh.Normals, int(mesh.VertexCount)*3)
			uv := unsafe.Slice(mesh.Texcoords, int(mesh.VertexCount)*2)
			for i := 0; i < int(mesh.VertexCount); i++ {
				x, y, z := v[i*3]/b.Size[0]+.5, v[i*3+1]/b.Size[1]+.5, v[i*3+2]/b.Size[2]+.5
				nx, ny, nz := n[i*3], n[i*3+1], n[i*3+2]
				w, h, d := b.Size[0]*16, b.Size[1]*16, b.Size[2]*16
				if b.UVAxis == "z" {
					y, z = z, 1-y
					ny, nz = nz, -ny
					h, d = d, h
				}
				u0, v0, uw, vh := b.UV[0], b.UV[1], w, h
				su, sv := x, 1-y
				switch {
				case ny > .5:
					u0 += d
					uw, vh = w, d
					su, sv = x, z
				case ny < -.5:
					u0 += d + w
					uw, vh = w, d
					su, sv = x, 1-z
				case nx > .5:
					v0 += d
					uw = d
					su = 1 - z
				case nx < -.5:
					u0 += d + w
					v0 += d
					uw = d
					su = z
				case nz > .5:
					u0 += d
					v0 += d
					su = 1 - x
				default:
					u0 += 2*d + w
					v0 += d
					su = x
				}
				uv[i*2] = (u0 + su*uw) / float32(tex.Width)
				uv[i*2+1] = (v0 + sv*vh) / float32(tex.Height)
			}
			rl.UpdateMeshBuffer(mesh, 1, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Texcoords)), int(mesh.VertexCount)*8), 0)
			m := rl.LoadModelFromMesh(mesh)
			rl.SetMaterialTexture(m.Materials, rl.MapDiffuse, tex)
			r.Models[id] = append(r.Models[id], m)
		}
	}
	return r, nil
}
func (r *MobRenderer) Close() {
	if r == nil {
		return
	}
	for _, models := range r.Models {
		for _, m := range models {
			rl.UnloadModel(m)
		}
	}
	for _, tex := range r.Textures {
		rl.UnloadTexture(tex)
	}
}
func mobPose(c *MobContent, kind string, phase, blend, death float32) []rl.Matrix {
	d := c.Definitions[kind]
	model := c.Models[d.Model]
	anim := c.Animations[d.Animation]
	angles := map[string]float32{}
	for _, ch := range anim.Channels {
		angles[ch.Bone] = float32(math.Sin(float64(phase+ch.Phase))) * anim.Amplitude * blend
	}
	poses := make([]rl.Matrix, len(model.Bones))
	byName := map[string]rl.Matrix{}
	for i, b := range model.Bones {
		local := rl.MatrixMultiply(rl.MatrixRotateX(angles[b.Name]), rl.MatrixTranslate(b.Pivot[0], b.Pivot[1], b.Pivot[2]))
		if b.Parent != "" {
			local = rl.MatrixMultiply(local, byName[b.Parent])
		}
		byName[b.Name] = local
		poses[i] = rl.MatrixMultiply(rl.MatrixTranslate(b.Offset[0], b.Offset[1], b.Offset[2]), local)
	}
	return poses
}
func (r *MobRenderer) Draw(e *RemoteEntity, debug bool) {
	d, ok := r.Content.Definitions[e.MobKind]
	if !ok {
		return
	}
	models := r.Models[d.Model]
	anim := r.Content.Animations[d.Animation]
	death := min(e.DeathTime/anim.DeathSeconds, float32(1))
	root := rl.MatrixMultiply(rl.MatrixMultiply(rl.MatrixRotateZ(death*math.Pi/2), rl.MatrixRotateY(e.Yaw)), rl.MatrixTranslate(float32(e.X), float32(e.Y)+death*.32, float32(e.Z)))
	tint := rl.White
	if e.MobHurt > 0 {
		tint = rl.NewColor(255, 115, 115, 255)
	}
	for i, pose := range mobPose(r.Content, e.MobKind, e.AnimPhase, e.AnimBlend, death) {
		m := models[i]
		m.Transform = rl.MatrixMultiply(pose, root)
		rl.DrawModel(m, rl.Vector3{}, 1, tint)
	}
	if debug {
		rl.DrawBoundingBox(mobBox(rl.NewVector3(float32(e.X), float32(e.Y), float32(e.Z)), d.Collider), rl.Green)
	}
}
