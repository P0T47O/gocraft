package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const entitySaveVersion = 10

func SaveEntities(savePath string, world *World) error {
	if err := ensureSaveDir(savePath); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString(entityMagic)
	buf.WriteByte(entitySaveVersion)

	world.entitiesMu.RLock()
	defer world.entitiesMu.RUnlock()

	entityCount := uint32(0)
	for _, e := range world.entities {
		if !transientEntity(e) {
			entityCount++
		}
	}
	writeUint32(&buf, entityCount)
	for _, e := range world.entities {
		if transientEntity(e) {
			continue
		}
		x, y, z := e.GetPosition()
		yaw, pitch := e.GetRotation()

		_ = WriteString(&buf, e.GetUUID())
		buf.WriteByte(byte(e.GetType()))
		_ = binary.Write(&buf, binary.LittleEndian, x)
		_ = binary.Write(&buf, binary.LittleEndian, y)
		_ = binary.Write(&buf, binary.LittleEndian, z)
		_ = binary.Write(&buf, binary.LittleEndian, yaw)
		_ = binary.Write(&buf, binary.LittleEndian, pitch)

		if m, ok := e.(*MobEntity); ok {
			b, err := json.Marshal(m)
			if err != nil {
				return err
			}
			if err = WriteString(&buf, string(b)); err != nil {
				return err
			}
		}
		if primed, ok := e.(*PrimedTNT); ok {
			_ = binary.Write(&buf, binary.LittleEndian, uint16(primed.Fuse))
		}

		// Save ItemEntity-specific data
		if item, ok := e.(*ItemEntity); ok {
			buf.WriteByte(byte(item.ID))
			_ = binary.Write(&buf, binary.LittleEndian, int32(item.Count))
			_ = binary.Write(&buf, binary.LittleEndian, item.Age)
			_ = binary.Write(&buf, binary.LittleEndian, item.Damage)
		}
	}

	path := filepath.Join(savePath, entityFile)
	return writeSaveFile(path, buf.Bytes())
}

func transientEntity(e Entity) bool {
	switch e.(type) {
	case *ArrowEntity:
		return true
	}
	return false
}

func LoadEntities(savePath string, world *World) (bool, error) {
	path := filepath.Join(savePath, entityFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(data) < 4+1+4 {
		return false, errors.New("entity save too small")
	}
	if string(data[:4]) != entityMagic {
		return false, errors.New("entity save magic mismatch")
	}

	if data[4] != 6 && data[4] != 7 && data[4] != 8 && data[4] != entitySaveVersion {
		return false, errors.New("unsupported entity version")
	}
	buf := bytes.NewBuffer(data[5:])
	count, err := ReadUint32(buf)
	if err != nil || count > uint32(len(data)/35) {
		return false, errors.New("invalid entity count")
	}

	world.entitiesMu.Lock()
	defer world.entitiesMu.Unlock()
	loaded := make([]Entity, 0, count)

	for i := uint32(0); i < count; i++ {
		uuid, err := ReadString(buf)
		if err != nil || buf.Len() < 33 {
			return false, errors.New("truncated entity")
		}
		etype, _ := buf.ReadByte()
		var x, y, z float64
		var yaw, pitch float32
		_ = binary.Read(buf, binary.LittleEndian, &x)
		_ = binary.Read(buf, binary.LittleEndian, &y)
		_ = binary.Read(buf, binary.LittleEndian, &z)
		_ = binary.Read(buf, binary.LittleEndian, &yaw)
		_ = binary.Read(buf, binary.LittleEndian, &pitch)

		// Entity Factory
		var e Entity
		switch EntityType(etype) {
		case EntityPig:
			if data[4] >= 9 {
				payload, err := ReadString(buf)
				if err != nil {
					return false, err
				}
				m := &MobEntity{}
				if err = json.Unmarshal([]byte(payload), m); err != nil {
					return false, err
				}
				def, ok := mobContent.Definitions[m.Kind]
				if !ok || m.Health < 0 || m.Health > 10000 || m.UUID != uuid || m.Type != EntityPig {
					return false, errors.New("invalid mob save")
				}
				m.Health = min(m.Health, def.Health)
				e = m
			} else {
				m := newMob("pig", uuid, x, y+.5, z)
				m.Yaw = yaw * math.Pi / 180
				e = m
			}
		case EntityPlayer:
			e = &PlayerEntity{
				BaseEntity: BaseEntity{
					UUID: uuid, Type: EntityType(etype),
					X: x, Y: y, Z: z,
					Yaw: yaw, Pitch: pitch,
				},
			}
		case EntityItem:
			if buf.Len() < 9 {
				return false, errors.New("truncated item")
			}
			// Read ItemEntity-specific data
			itemID, _ := buf.ReadByte()
			var count int32
			var age float32
			_ = binary.Read(buf, binary.LittleEndian, &count)
			_ = binary.Read(buf, binary.LittleEndian, &age)
			var damage int32
			if data[4] >= 8 {
				if err := binary.Read(buf, binary.LittleEndian, &damage); err != nil {
					return false, err
				}
			}
			e = &ItemEntity{
				BaseEntity: BaseEntity{
					UUID: uuid, Type: EntityType(etype),
					X: x, Y: y, Z: z,
					Yaw: yaw, Pitch: pitch,
				},
				ItemStack: ItemStack{ID: int32(itemID), Count: count, Damage: damage},
				Age:       age,
			}
		case EntityPrimedTNT:
			if data[4] < 10 || buf.Len() < 2 {
				return false, errors.New("truncated primed TNT")
			}
			var fuse uint16
			if err := binary.Read(buf, binary.LittleEndian, &fuse); err != nil || fuse == 0 || fuse > tntFuseTicks {
				return false, errors.New("invalid TNT fuse")
			}
			e = &PrimedTNT{BaseEntity: BaseEntity{UUID: uuid, Type: EntityPrimedTNT, X: x, Y: y, Z: z}, Fuse: int(fuse)}
		default:
			e = &BaseEntity{
				UUID: uuid, Type: EntityType(etype),
				X: x, Y: y, Z: z,
				Yaw: yaw, Pitch: pitch,
			}
		}
		if item, ok := e.(*ItemEntity); ok {
			one := item.ItemStack
			one.Count = 1
			if !validStack(one) || item.Count <= 0 || item.Count > 64 {
				return false, errors.New("invalid saved item")
			}
			if data[4] >= 8 && !validStack(item.ItemStack) {
				return false, errors.New("invalid saved stack")
			}
			for item.Count > StackLimit(item.ID) {
				extra := *item
				extra.UUID = fmt.Sprintf("%s-migrated-%d", item.UUID, item.Count)
				extra.Count = StackLimit(item.ID)
				item.Count -= extra.Count
				loaded = append(loaded, &extra)
			}
		}
		loaded = append(loaded, e)
	}

	if buf.Len() != 0 {
		return false, errors.New("unexpected entity data")
	}
	world.entities = loaded
	return true, nil
}
