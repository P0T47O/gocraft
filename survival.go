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
	data := survivalPlayerSave{Version: 3, Players: make(map[string]PlayerEntity)}
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
	return writeSaveFile(path, b)
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
	if data.Version != 1 && data.Version != 2 && data.Version != 3 {
		return fmt.Errorf("unsupported player save version %d", data.Version)
	}
	for name, p := range data.Players {
		if name == "" || p.SelectedSlot < 0 || p.SelectedSlot >= 9 || p.GameMode > ModeSurvival {
			return fmt.Errorf("invalid player state for %q", name)
		}
		if v := p.Vitals; v != nil {
			if data.Version < 3 {
				v.Food, v.Saturation = maxFood, 5
			}
			// Natural recovery adds 3 exhaustion after the tick's drain pass,
			// so 4..7 is a valid transient save state.
			if v.Health < 0 || v.Health > maxHealth || v.Air < 0 || v.Air > maxAir || v.FireTicks < 0 || v.FireTicks > 160 || v.FallDistance < 0 || v.Food < 0 || v.Food > maxFood || math.IsNaN(v.Saturation) || math.IsInf(v.Saturation, 0) || v.Saturation < 0 || v.Saturation > float64(v.Food) || math.IsNaN(v.Exhaustion) || math.IsInf(v.Exhaustion, 0) || v.Exhaustion < 0 || v.Exhaustion >= 7 {
				return fmt.Errorf("invalid vitals for %q", name)
			}
		}
		if data.Version == 1 {
			for _, stack := range append(p.Inventory.Slots[:], p.CursorItem) {
				if stack.ID < 0 || stack.ID > 255 || stack.Count < 0 || stack.Count > 64 || (stack.ID == 0) != (stack.Count == 0) {
					return fmt.Errorf("invalid legacy inventory for %q", name)
				}
				if stack.ID != 0 && Items[stack.ID] == nil {
					return fmt.Errorf("unknown legacy item")
				}
			}
			migratePlayerItems(&p)
		}
		for _, stack := range append(p.Inventory.Slots[:], p.CursorItem) {
			if !validStack(stack) {
				return fmt.Errorf("invalid inventory for %q", name)
			}
		}
		for _, stack := range p.PendingItems {
			one := stack
			one.Count = 1
			if stack.Count < 1 || stack.Count > 64 || !validStack(one) {
				return fmt.Errorf("invalid migration overflow")
			}
		}
		data.Players[name] = p

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
