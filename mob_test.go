package main

import (
	"bytes"
	"encoding/binary"
	rl "github.com/gen2brain/raylib-go/raylib"
	"io/fs"
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
			c.blocks[x][70][z] = blockStone
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
	pose := mobPose(c, "pig", 1, 0, 0)
	walking := mobPose(c, "pig", 1, 1, 0)
	if reflect.DeepEqual(pose, walking) || pose[0] != walking[0] {
		t.Fatal("animation did not isolate leg joints")
	}
	p := "content/animations/quadruped.json"
	source[p] = &fstest.MapFile{Data: bytes.ReplaceAll(source[p].Data, []byte("leg_front_left"), []byte("missing_leg"))}
	if _, err = loadMobContent(source); err == nil {
		t.Fatal("missing animation bone accepted")
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
	eye := rl.NewVector3(8, 70.501+playerEyeY, 8)
	moved := resolveCollision(w, eye, rl.NewVector3(0, -5, 0))
	if abs32(moved.Y-eye.Y) > .002 {
		t.Fatal("player eye/feet adapter changed landing")
	}
}
