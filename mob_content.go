package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
)

//go:embed content/entities/*.json content/models/*.json content/animations/*.json
var bundledMobs embed.FS

type MobDefinition struct {
	SpawnLimit  int      `json:"spawn_limit"`
	SpawnRadius float64  `json:"spawn_radius"`
	ID          string   `json:"id"`
	Health      int      `json:"health"`
	Speed       float32  `json:"speed"`
	FleeSpeed   float32  `json:"flee_speed"`
	Collider    Collider `json:"collider"`
	Model       string   `json:"model"`
	Animation   string   `json:"animation"`
	Behaviors   []string `json:"behaviors"`
	IdleTicks   int      `json:"idle_ticks"`
	WalkTicks   int      `json:"walk_ticks"`
	FleeTicks   int      `json:"flee_ticks"`
	Drop        string   `json:"drop"`
	DropMin     int32    `json:"drop_min"`
	DropMax     int32    `json:"drop_max"`
}
type MobBone struct {
	UVAxis              string `json:"uv_axis"`
	Name, Parent        string
	Pivot, Size, Offset [3]float32
	UV                  [2]float32
}
type MobModel struct {
	Texture     string     `json:"texture"`
	TextureSize [2]float32 `json:"texture_size"`
	Bones       []MobBone  `json:"bones"`
}
type MobAnimation struct {
	Stride, Amplitude float32
	BlendSpeed        float32 `json:"blend_speed"`
	DeathSeconds      float32 `json:"death_seconds"`
	Channels          []struct {
		Bone  string
		Phase float32
	}
}
type MobContent struct {
	Definitions map[string]MobDefinition
	Models      map[string]MobModel
	Animations  map[string]MobAnimation
}

var mobContent = mustMobContent()

func mustMobContent() *MobContent {
	c, err := loadMobContent(bundledMobs)
	if err != nil {
		panic(err)
	}
	return c
}
func loadMobContent(source fs.FS) (*MobContent, error) {
	c := &MobContent{Definitions: map[string]MobDefinition{}, Models: map[string]MobModel{}, Animations: map[string]MobAnimation{}}
	read := func(path string, out any) error {
		b, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	}
	files, err := fs.Glob(source, "content/entities/*.json")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no mob definitions")
	}
	for _, path := range files {
		var d MobDefinition
		if err = read(path, &d); err != nil {
			return nil, err
		}
		if d.ID == "" || d.SpawnLimit < 0 || d.SpawnLimit > 32 || d.SpawnRadius < 8 || d.SpawnRadius > 64 || d.Health <= 0 || d.Health > 10000 || d.Speed <= 0 || d.Speed > 10 || d.FleeSpeed <= 0 || d.FleeSpeed > 15 || d.IdleTicks < 1 || d.WalkTicks < 1 || d.FleeTicks < 1 || d.Collider.Width <= 0 || d.Collider.Width > 4 || d.Collider.Depth <= 0 || d.Collider.Depth > 4 || d.Collider.Height <= 0 || d.Collider.Height > 4 || d.Collider.StepHeight < 0 || d.Collider.StepHeight > 1 || d.Drop != "raw_pork" || d.DropMin < 1 || d.DropMax < d.DropMin || d.DropMax > 64 {
			return nil, fmt.Errorf("%s: invalid attributes", path)
		}
		if _, exists := c.Definitions[d.ID]; exists {
			return nil, fmt.Errorf("duplicate mob %s", d.ID)
		}
		for _, behavior := range d.Behaviors {
			if behavior != "wander" && behavior != "flee" {
				return nil, fmt.Errorf("%s: unknown behavior %s", path, behavior)
			}
		}
		var model MobModel
		var anim MobAnimation
		if err = read("content/models/"+d.Model+".json", &model); err != nil {
			return nil, err
		}
		if err = read("content/animations/"+d.Animation+".json", &anim); err != nil {
			return nil, err
		}
		if model.Texture == "" || model.TextureSize[0] <= 0 || model.TextureSize[1] <= 0 || len(model.Bones) == 0 || len(model.Bones) > 64 {
			return nil, fmt.Errorf("model %s: invalid texture/bones", d.Model)
		}
		bones := map[string]bool{}
		for _, b := range model.Bones {
			if b.Name == "" || bones[b.Name] || (b.Parent != "" && !bones[b.Parent]) {
				return nil, fmt.Errorf("model %s: duplicate bone or parent must precede child: %s", d.Model, b.Name)
			}
			for _, v := range b.Size {
				if v <= 0 || v > 8 {
					return nil, fmt.Errorf("model %s: invalid size", d.Model)
				}
			}
			if b.UVAxis != "" && b.UVAxis != "z" {
				return nil, fmt.Errorf("model %s: unknown UV axis", d.Model)
			}
			width, height, depth := b.Size[0]*16, b.Size[1]*16, b.Size[2]*16
			if b.UVAxis == "z" {
				height, depth = depth, height
			}
			if b.UV[0] < 0 || b.UV[1] < 0 || b.UV[0]+2*(width+depth) > model.TextureSize[0]+.001 || b.UV[1]+depth+height > model.TextureSize[1]+.001 {
				return nil, fmt.Errorf("model %s bone %s: UV outside texture", d.Model, b.Name)
			}
			bones[b.Name] = true
		}
		if anim.Stride <= 0 || anim.BlendSpeed <= 0 || anim.DeathSeconds <= 0 || math.IsNaN(float64(anim.Amplitude)) {
			return nil, fmt.Errorf("animation %s: invalid settings", d.Animation)
		}
		for _, ch := range anim.Channels {
			if !bones[ch.Bone] {
				return nil, fmt.Errorf("animation %s: missing bone %s", d.Animation, ch.Bone)
			}
		}
		c.Definitions[d.ID] = d
		c.Models[d.Model] = model
		c.Animations[d.Animation] = anim
	}
	if _, ok := c.Definitions["pig"]; !ok {
		return nil, fmt.Errorf("pig definition required for legacy worlds")
	}
	return c, nil
}

// Preview reload is transactional; simulation definitions remain fixed for a running world.
func reloadMobPreview() (*MobContent, error) { return loadMobContent(os.DirFS(".")) }

func loadRuntimeMobs() error {
	if _, err := os.Stat("content/entities"); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	c, err := reloadMobPreview()
	if err != nil {
		return err
	}
	mobContent = c
	return nil
}
