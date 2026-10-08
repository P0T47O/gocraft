package main

import (
	"bytes"
	"encoding/binary"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func mobTestWorld(t *testing.T) *World {
	t.Helper()
	initBlockRegistry()
	w := NewClientWorld()
	t.Cleanup(w.Close)
	c := lifecycleChunk(w, chunkKey{0, 0})
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.blocks.Set(x, 70, z, blockStone)
		}
	}
	return w
}
func TestMobContentValidationAndPose(t *testing.T) {
	source := fstest.MapFS{}
	fs.WalkDir(bundledMobs, "content", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := fs.ReadFile(bundledMobs, path)
			source[path] = &fstest.MapFile{Data: b}
		}
		return err
	})
	c, err := loadMobContent(source)
	if err != nil {
		t.Fatal(err)
	}
	definition := c.Definitions["pig"]
	model, animation := c.Models[definition.Model], c.Animations[definition.Animation]
	pose := evaluateMobPose(model, animation, 1, 0, nil)
	walking := evaluateMobPose(model, animation, 1, 1, nil)
	if reflect.DeepEqual(pose, walking) || pose[0] != walking[0] {
		t.Fatal("animation did not isolate leg joints")
	}
	p := "content/animations/quadruped.json"
	source[p] = &fstest.MapFile{Data: bytes.ReplaceAll(source[p].Data, []byte("leg_front_left"), []byte("missing_leg"))}
	if _, err = loadMobContent(source); err == nil {
		t.Fatal("missing animation bone accepted")
	}
}

func TestMobVisualModelLayout(t *testing.T) {
	chicken := mobContent.Models[mobContent.Definitions["chicken"].Model]
	legs, wings := 0, 0
	for _, bone := range chicken.Bones {
		if bone.Name == "leg_left" || bone.Name == "leg_right" {
			legs++
		}
		if bone.Name == "wing_left" || bone.Name == "wing_right" {
			wings++
		}
	}
	if legs != 2 || wings != 2 || mobContent.Definitions["chicken"].Animation != "chicken" {
		t.Fatalf("chicken model: %d legs, %d wings, animation %q", legs, wings, mobContent.Definitions["chicken"].Animation)
	}
	sheep := mobContent.Models[mobContent.Definitions["sheep"].Model]
	wool := 0
	for _, bone := range sheep.Bones {
		if bone.Texture == "textures/block/white_wool.png" {
			wool++
			if !bone.HideWhenSheared || !bone.FullTexture {
				t.Fatalf("sheep wool bone %s does not disappear when sheared", bone.Name)
			}
		}
	}
	if wool != 6 {
		t.Fatalf("sheep wool layer has %d bones, want body, head and four leg cuffs", wool)
	}
	spider := mobContent.Models[mobContent.Definitions["spider"].Model]
	upperLegs, lowerLegs := 0, 0
	spiderBones := make(map[string]MobBone, len(spider.Bones))
	for _, bone := range spider.Bones {
		spiderBones[bone.Name] = bone
		if len(bone.Name) >= 4 && bone.Name[:4] == "leg_" {
			if bone.SegmentTo == ([3]float32{}) || bone.SegmentTo[0] == 0 {
				t.Fatalf("spider leg %s must have a diagonal segment: %v", bone.Name, bone.SegmentTo)
			}
			if bone.Parent == "" {
				upperLegs++
			} else {
				lowerLegs++
				if bone.Pivot != spiderBones[bone.Parent].SegmentTo {
					t.Fatalf("spider leg %s knee does not meet %s", bone.Name, bone.Parent)
				}
				if bone.SegmentTo[1] >= -.5 {
					t.Fatalf("spider lower leg %s does not reach the ground", bone.Name)
				}
			}
		}
	}
	if upperLegs != 8 || lowerLegs != 8 {
		t.Fatalf("spider has %d upper and %d lower legs, want 8 each", upperLegs, lowerLegs)
	}
}
func TestMobCollisionStepAndCliff(t *testing.T) {
	w := mobTestWorld(t)
	m := newMob("pig", "physics", 8, 70.501, 3)
	m.State = "walk"
	m.Timer = 1000
	w.SetBlockAt(8, 71, 5, blockStone)
	w.SetBlockAt(8, 72, 5, blockStone)
	for i := 0; i < 35; i++ {
		m.Yaw = 0
		m.Tick(w)
	}
	if m.Z > 3.81 || m.Y > 70.51 {
		t.Fatal("mob crossed two-block wall", m.X, m.Y, m.Z)
	}
	w.SetBlockAt(8, 72, 5, blockAir)
	m.State = "walk"
	m.Timer = 1000
	for i := 0; i < 35; i++ {
		m.Yaw = 0
		m.Tick(w)
	}
	if m.Y < 71.49 {
		t.Fatal("one-block step not climbed", m.Y, m.Z)
	}
	m = newMob("pig", "cliff", 8, 70.501, 8)
	m.State = "walk"
	m.Timer = 1000
	for x := 0; x < 16; x++ {
		for z := 10; z < 16; z++ {
			w.SetBlockAt(x, 70, z, blockAir)
		}
	}
	for i := 0; i < 80; i++ {
		m.Yaw = 0
		m.Tick(w)
	}
	if m.Y < 70.49 {
		t.Fatal("walked off cliff", m.Y, m.Z)
	}
	// Unloaded neighbors must freeze, never snap to procedural height.
	m.X = 16
	before := m.Y
	m.Tick(w)
	if m.Y != before {
		t.Fatal("unloaded terrain advanced gravity")
	}
}

func TestPassiveMobStopsTurningAtObstacle(t *testing.T) {
	w := mobTestWorld(t)
	w.SetBlockAt(8, 71, 5, blockStone)
	w.SetBlockAt(8, 72, 5, blockStone)
	m := newMob("pig", "blocked-turn", 8, 70.501, 3)
	m.State, m.Timer, m.Yaw = "walk", 1000, 0
	for i := 0; i < 30 && m.State == "walk"; i++ {
		m.Tick(w)
	}
	if m.State != "turn" {
		t.Fatal("pig did not pause after hitting the wall")
	}
	position := [2]float64{m.X, m.Z}
	yaw := m.Yaw
	for i := 0; i < 5; i++ {
		m.Tick(w)
		if m.State != "turn" || abs32(m.Yaw-yaw) > .121 {
			t.Fatalf("pig turned too sharply or resumed walking while blocked: state=%s yaw=%f", m.State, m.Yaw)
		}
		yaw = m.Yaw
	}
	if m.X != position[0] || m.Z != position[1] {
		t.Fatal("pig moved while turning in place")
	}
}

func TestPassiveMobTurnsBeforeWandering(t *testing.T) {
	w := mobTestWorld(t)
	m := newMob("pig", "wander-turn", 8, 70.501, 8)
	m.State, m.Timer = "idle", 1
	m.Tick(w)
	if m.State != "turn" || m.X != 8 || m.Z != 8 {
		t.Fatalf("pig started walking before turning: state=%s pos=(%f,%f)", m.State, m.X, m.Z)
	}
	for i := 0; i < 30; i++ {
		previousYaw := m.Yaw
		oldX, oldZ := m.X, m.Z
		m.Tick(w)
		if m.State == "turn" {
			if math.Abs(float64(m.Yaw-previousYaw)) > .121 || m.X != oldX || m.Z != oldZ {
				t.Fatal("pig moved or snapped its heading while turning")
			}
			continue
		}
		if m.State != "walk" || math.Hypot(m.X-oldX, m.Z-oldZ) < .001 {
			t.Fatal("pig did not resume walking after its turn")
		}
		motionYaw := math.Atan2(m.X-oldX, m.Z-oldZ)
		if math.Cos(motionYaw-float64(m.Yaw)) < .99 {
			t.Fatalf("pig walked away from its facing: heading=%f motion=%f", m.Yaw, motionYaw)
		}
		return
	}
	t.Fatal("pig never finished turning")
}
func TestMobAttackCooldownWallAndDrops(t *testing.T) {
	w := mobTestWorld(t)
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "attacker", Type: EntityPlayer, X: 8, Y: 71.1, Z: 4}, GameMode: ModeSurvival}
	p.Inventory.Slots[0] = Item{ID: int32(itemWoodAxe), Count: 1, Damage: 4}
	m := newMob("pig", "victim", 8, 70.501, 6)
	w.entities = []Entity{p, m}
	s := &Server{World: w, Clients: map[string]*ClientConnection{}}
	w.SetBlockAt(8, 71, 5, blockStone)
	s.attackMob(p, m.UUID)
	if m.Health != 10 {
		t.Fatal("attack crossed wall")
	}
	w.SetBlockAt(8, 71, 5, blockAir)
	s.attackMob(p, m.UUID)
	if m.Health != 6 || p.Inventory.Slots[0].Damage != 5 || m.Flee == 0 {
		t.Fatal("attack did not apply damage/wear/flee", m.Health)
	}
	s.attackMob(p, m.UUID)
	if m.Health != 6 {
		t.Fatal("attack cooldown bypassed")
	}
	m.Health = 0
	s.updateMobs()
	s.updateMobs()
	drops := 0
	for _, e := range w.entities {
		if d, ok := e.(*ItemEntity); ok {
			drops++
			if d.ID != int32(itemRawPork) || d.Count < 1 || d.Count > 3 {
				t.Fatal("invalid mob loot")
			}
		}
	}
	if drops != 1 {
		t.Fatal("death duplicated drops", drops)
	}
}
func TestMobSaveAndLegacyMigration(t *testing.T) {
	w := mobTestWorld(t)
	m := newMob("pig", "saved", 8, 70.501, 8)
	m.Health = 3
	m.State = "flee"
	m.Flee = 40
	m.Yaw = 1.2
	m.Velocity.Y = 2
	w.entities = []Entity{m}
	root := t.TempDir()
	if err := SaveEntities(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if _, err := LoadEntities(root, loaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, loaded.entities[0]) {
		t.Fatal("mob state not persisted")
	}
	var b bytes.Buffer
	b.WriteString(entityMagic)
	b.WriteByte(8)
	writeUint32(&b, 1)
	WriteString(&b, "old-pig")
	b.WriteByte(byte(EntityPig))
	for _, v := range []float64{8, 70, 8} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	binary.Write(&b, binary.LittleEndian, float32(90))
	binary.Write(&b, binary.LittleEndian, float32(0))
	os.WriteFile(filepath.Join(root, entityFile), b.Bytes(), 0600)
	if _, err := LoadEntities(root, loaded); err != nil {
		t.Fatal(err)
	}
	old := loaded.entities[0].(*MobEntity)
	if old.Health != 10 || old.Y != 70.5 || abs32(old.Yaw-1.5707963) > .001 {
		t.Fatal("legacy origin/angle conversion failed", old)
	}
	var wire bytes.Buffer
	packet := m.snapshot()
	WritePacket(&wire, packet)
	got, err := ReadPacket(&wire)
	if err != nil || !reflect.DeepEqual(packet, got) {
		t.Fatal("mob packet roundtrip", err)
	}
}
func TestSharedColliderKeepsPlayerOrigin(t *testing.T) {
	w := mobTestWorld(t)
	eye := newGameVec3(8, 70.501+playerEyeY, 8)
	moved := resolveCollision(w, eye, newGameVec3(0, -5, 0))
	if abs32(moved.Y-eye.Y) > .002 {
		t.Fatal("player eye/feet adapter changed landing")
	}
}
