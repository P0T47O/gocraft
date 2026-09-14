package main

import (
	"encoding/json"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"strings"
)

func isAnimatedTexture(path string) bool {
	switch path {
	case "textures/block/water_still.png", "textures/block/water_flow.png", "textures/block/lava_still.png", "textures/block/lava_flow.png":
		return true
	default:
		return false
	}
}

func (a *RenderAssets) loadAnimatedTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[anim.Index]
	}
	img := rl.LoadImage(path)
	if img == nil || img.Data == nil {
		if img != nil {
			rl.UnloadImage(img)
		}
		return rl.Texture2D{}
	}
	w := int(img.Width)
	h := int(img.Height)
	frameSize := w
	if h < w {
		frameSize = h
	}
	frames := 0
	if frameSize > 0 {
		frames = h / frameSize
	}
	if frames <= 0 {
		frames = 1
	}
	frameList := make([]rl.Texture2D, 0, frames)
	for i := 0; i < frames; i++ {
		frame := rl.ImageCopy(img)
		rec := rl.NewRectangle(0, float32(i*frameSize), float32(frameSize), float32(frameSize))
		rl.ImageCrop(frame, rec)
		tex := rl.LoadTextureFromImage(frame)
		if tex.ID != 0 {
			rl.SetTextureFilter(tex, rl.FilterPoint)
			frameList = append(frameList, tex)
		}
		rl.UnloadImage(frame)
	}
	rl.UnloadImage(img)

	frameSeconds := float32(0.1)
	metaPath := path + ".mcmeta"
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta struct {
			Animation struct {
				FrameTime int `json:"frametime"`
			} `json:"animation"`
		}
		if json.Unmarshal(data, &meta) == nil && meta.Animation.FrameTime > 0 {
			frameSeconds = float32(meta.Animation.FrameTime) / 20.0
		}
	}
	anim := &AnimatedTexture{
		Frames:       frameList,
		FrameSeconds: frameSeconds,
		Time:         0,
		Index:        0,
	}
	a.animated[path] = anim
	if len(frameList) == 0 {
		return rl.Texture2D{}
	}
	return frameList[0]
}

func (a *RenderAssets) Update(dt float32) {
	for path, anim := range a.animated {
		if len(anim.Frames) <= 1 || anim.FrameSeconds <= 0 {
			continue
		}
		index := int(rl.GetTime()/float64(anim.FrameSeconds)) % len(anim.Frames)
		if index != anim.Index {
			anim.Index = index
			a.setTextureForPath(path, anim.Frames[anim.Index])
		}
	}
}

func (a *RenderAssets) setTextureForPath(path string, tex rl.Texture2D) {
	a.textures[path] = tex
	for key, model := range a.faceModels {
		if strings.HasSuffix(key, "|"+path) {
			materials := model.GetMaterials()
			if len(materials) > 0 {
				rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
			}
			a.faceModels[key] = model
		}
	}
}

func (a *RenderAssets) loadBlockTextures() {
	for i := 0; i < 256; i++ {
		def := GetItemVisual(byte(i))
		if def == nil || def.ID == blockAir {
			continue
		}
		a.loadTexture(def.Textures.Top)
		a.loadTexture(def.Textures.Bottom)
		a.loadTexture(def.Textures.North)
		a.loadTexture(def.Textures.South)
		a.loadTexture(def.Textures.East)
		a.loadTexture(def.Textures.West)
	}
}

func (a *RenderAssets) getFaceModelAnimated(face string, path string) rl.Model {
	model := a.getFaceModel(face, path)
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		tex := anim.Frames[anim.Index]
		materials := model.GetMaterials()
		if len(materials) > 0 {
			rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
		}
	}
	return model
}

func (a *RenderAssets) isAnimated(path string) bool {
	anim, ok := a.animated[path]
	return ok && len(anim.Frames) > 1
}

func (a *RenderAssets) currentTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[anim.Index]
	}
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	return rl.Texture2D{}
}

func (a *RenderAssets) baseTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[0]
	}
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	return rl.Texture2D{}
}

func (a *RenderAssets) applyTextureToModel(model rl.Model, tex rl.Texture2D) rl.Model {
	if tex.ID == 0 {
		return model
	}
	materials := model.GetMaterials()
	if len(materials) > 0 {
		rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
	}
	return model
}
