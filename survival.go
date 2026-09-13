package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Separate versioning preserves compatibility with existing chunk/entity files.
type survivalPlayerSave struct {
	Version int
	Players map[string]PlayerEntity
}

func saveSurvivalPlayers(root string, world *World) error {
	data := survivalPlayerSave{Version: 1, Players: make(map[string]PlayerEntity)}
	world.entitiesMu.RLock()
	for _, e := range world.entities {
		if p, ok := e.(*PlayerEntity); ok {
			data.Players[p.UUID] = *p
		}
	}
	world.entitiesMu.RUnlock()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return err
	}
	path := filepath.Join(root, "players.json")
	f, err := os.CreateTemp(root, "players-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func loadSurvivalPlayers(root string, world *World) error {
	b, err := os.ReadFile(filepath.Join(root, "players.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var data survivalPlayerSave
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}
	if data.Version != 1 {
		return fmt.Errorf("unsupported player save version %d", data.Version)
	}
	for name, p := range data.Players {
		if name == "" || p.SelectedSlot < 0 || p.SelectedSlot >= 9 || p.GameMode > ModeSurvival {
			return fmt.Errorf("invalid player state for %q", name)
		}
		if v := p.Vitals; v != nil {
			if v.Health < 0 || v.Health > maxHealth || v.Air < 0 || v.Air > maxAir || v.FireTicks < 0 || v.FireTicks > 160 || v.FallDistance < 0 {
				return fmt.Errorf("invalid vitals for %q", name)
			}
		}
		for _, item := range append(p.Inventory.Slots[:], p.CursorItem) {
			if item.ID < 0 || item.ID > 255 || item.Count < 0 || item.Count > MaxStackSize || (item.ID == 0) != (item.Count == 0) {
				return fmt.Errorf("invalid inventory for %q", name)
			}
		}
	}
	// The old entity file contains player positions but no inventories.
	for name, saved := range data.Players {
		p := saved
		p.UUID = name
		p.Type = EntityPlayer
		found := false
		for i, e := range world.entities {
			if e.GetUUID() == name && e.GetType() == EntityPlayer {
				world.entities[i] = &p
				found = true
				break
			}
		}
		if !found {
			world.entities = append(world.entities, &p)
		}
	}
	return nil
}

func (s *Server) hasCraftingStation(p *PlayerEntity, station byte) bool {
	if station == 0 {
		return true
	}
	x, y, z := int(math.Floor(p.X+0.5)), int(math.Floor(p.Y+0.5)), int(math.Floor(p.Z+0.5))
	for dx := -5; dx <= 5; dx++ {
		for dy := -5; dy <= 5; dy++ {
			for dz := -5; dz <= 5; dz++ {
				if dx*dx+dy*dy+dz*dz <= 25 && s.World.BlockAt(x+dx, y+dy, z+dz) == station {
					return true
				}
			}
		}
	}
	return false
}

func harvestTier(m ToolMaterial) int {
	switch m {
	case MatWood, MatGold:
		return 1
	case MatStone:
		return 2
	case MatIron:
		return 3
	case MatDiamond:
		return 4
	}
	return 0
}
