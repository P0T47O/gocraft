package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

type BlockPos struct{ X, Y, Z int32 }
type BlockContainer struct {
	Pos                   BlockPos
	Kind                  byte
	Slots                 []ItemStack
	Burn, BurnTotal, Cook int32
	CookingID             int32
	Revision              int32
}
type ContainerSession struct {
	Pos   BlockPos
	Token int32
}

func containerSize(kind byte) int {
	if kind == blockChest {
		return 27
	}
	if kind == blockFurnace {
		return 3
	}
	return 0
}
func smeltResult(id int32) int32 {
	switch byte(id) {
	case itemRawPork:
		return int32(itemCookedPork)
	case blockIronOre:
		return int32(itemIronIngot)
	case blockGoldOre:
		return int32(itemGoldIngot)
	case blockSand:
		return int32(blockGlass)
	case blockCobblestone:
		return int32(blockStone)
	}
	return 0
}
func fuelTicks(id int32) int32 {
	switch byte(id) {
	case itemCoal:
		return 1600
	case blockCoalBlock:
		return 16000
	case blockLog, blockLogBirch, blockLogSpruce, blockPlank, blockPlankOak, blockPlankBirch, blockPlankSpruce:
		return 300
	case itemStick:
		return 100
	}
	return 0
}

// 20 server ticks per second; one item takes ten seconds.
func (c *BlockContainer) tick() {
	if c.Kind != blockFurnace {
		return
	}
	if c.Burn > 0 {
		c.Burn--
	}
	input, fuel, output := &c.Slots[0], &c.Slots[1], &c.Slots[2]
	result := smeltResult(input.ID)
	if c.CookingID != input.ID {
		c.Cook = 0
		c.CookingID = input.ID
	}
	canCook := input.Count > 0 && result != 0 && (output.ID == 0 || CanStack(*output, Item{ID: result, Count: 1})) && output.Count < StackLimit(result)
	if !canCook {
		c.Cook = 0
		return
	}
	if c.Burn == 0 && fuel.Count > 0 && fuelTicks(fuel.ID) > 0 {
		c.BurnTotal = fuelTicks(fuel.ID)
		c.Burn = c.BurnTotal
		fuel.Count--
		if fuel.Count == 0 {
			*fuel = Item{}
		}
	}
	if c.Burn == 0 {
		c.Cook = max(0, c.Cook-2)
		return
	}
	c.Cook++
	if c.Cook >= 200 {
		product := Item{ID: result, Count: 1}
		MoveStack(output, &product, 1)
		input.Count--
		if input.Count == 0 {
			*input = Item{}
		}
		c.Cook = 0
	}
}

func (s *Server) containerReach(p *PlayerEntity, pos BlockPos) bool {
	if p == nil || p.dead() {
		return false
	}
	dx, dy, dz := p.X-float64(pos.X), p.Y-float64(pos.Y), p.Z-float64(pos.Z)
	return !math.IsNaN(dx+dy+dz) && dx*dx+dy*dy+dz*dz <= 36
}
func (s *Server) openContainer(p *PlayerEntity, pos BlockPos) {
	if !s.containerReach(p, pos) {
		return
	}
	kind := s.World.BlockAt(int(pos.X), int(pos.Y), int(pos.Z))
	n := containerSize(kind)
	if n == 0 {
		return
	}
	if s.Containers == nil {
		s.Containers = make(map[BlockPos]*BlockContainer)
	}
	if s.ContainerSessions == nil {
		s.ContainerSessions = make(map[string]ContainerSession)
	}
	c := s.Containers[pos]
	if c == nil {
		c = &BlockContainer{Pos: pos, Kind: kind, Slots: make([]ItemStack, n)}
		s.Containers[pos] = c
	}
	if c.Kind != kind {
		return
	}
	s.ContainerToken++
	if s.ContainerToken <= 0 {
		s.ContainerToken = 1
	}
	s.ContainerSessions[p.UUID] = ContainerSession{Pos: pos, Token: s.ContainerToken}
	s.sendContainer(p.UUID)
	s.sendContainerInventory(p.UUID)
}
func (s *Server) sendContainer(name string) {
	session, ok := s.ContainerSessions[name]
	if !ok {
		return
	}
	c := s.Containers[session.Pos]
	if c == nil {
		return
	}
	snapshot := *c
	snapshot.Slots = append([]ItemStack(nil), c.Slots...)
	s.SendTo(name, &PacketContainerState{Token: session.Token, State: snapshot})
}
func (s *Server) syncContainer(pos BlockPos) {
	for name, session := range s.ContainerSessions {
		if session.Pos == pos {
			s.sendContainer(name)
		}
	}
}
func (s *Server) closeContainer(name string) {
	if session, ok := s.ContainerSessions[name]; ok {
		delete(s.ContainerSessions, name)
		s.SendTo(name, &PacketContainerState{Token: session.Token})
	}
}
func acceptsContainerItem(c *BlockContainer, index int, stack ItemStack) bool {
	if stack.ID == 0 {
		return true
	}
	if c.Kind == blockChest {
		return true
	}
	if index == 0 {
		return smeltResult(stack.ID) != 0
	}
	return index == 1 && fuelTicks(stack.ID) > 0
}

// A click is processed entirely on the server. Revision checks reject stale views.
func (s *Server) clickContainer(p *PlayerEntity, packet *PacketContainerClick) {
	if p == nil {
		return
	}
	session, ok := s.ContainerSessions[p.UUID]
	if !ok || session.Token != packet.Token {
		return
	}
	if packet.Slot == -1 {
		s.closeContainer(p.UUID)
		return
	}
	c := s.Containers[session.Pos]
	if c == nil || !s.containerReach(p, session.Pos) || s.World.BlockAt(int(session.Pos.X), int(session.Pos.Y), int(session.Pos.Z)) != c.Kind {
		s.closeContainer(p.UUID)
		return
	}
	if c.Revision != packet.Revision {
		s.sendContainer(p.UUID)
		s.sendContainerInventory(p.UUID)
		return
	}
	index := int(packet.Slot)
	n := len(c.Slots)
	if index < 0 || index >= n+36 || packet.Button < 0 || packet.Button > 2 {
		return
	}
	var slot *ItemStack
	if index < n {
		slot = &c.Slots[index]
	} else {
		slot = &p.Inventory.Slots[index-n]
	}
	cursor := &p.CursorItem
	if packet.Button == 2 {
		if cursor.ID != 0 {
			return
		}
		if index < n {
			slot.Count = p.Inventory.AddStack(*slot)
			if slot.Count == 0 {
				*slot = Item{}
			}
		} else {
			for pass := 0; pass < 2; pass++ {
				for i := range c.Slots {
					dst := &c.Slots[i]
					if (pass == 0 && dst.ID == 0) || (pass == 1 && dst.ID != 0) || !acceptsContainerItem(c, i, *slot) {
						continue
					}
					MoveStack(dst, slot, slot.Count)
				}
			}
		}
	} else if c.Kind == blockFurnace && index == 2 {
		amount := slot.Count
		if packet.Button == 1 {
			amount = (amount + 1) / 2
		}
		MoveStack(cursor, slot, amount)
	} else if cursor.ID == 0 {
		amount := slot.Count
		if packet.Button == 1 {
			amount = (amount + 1) / 2
		}
		MoveStack(cursor, slot, amount)
	} else if index >= n || acceptsContainerItem(c, index, *cursor) {
		if packet.Button == 0 && slot.ID != 0 && !CanStack(*slot, *cursor) {
			*slot, *cursor = *cursor, *slot
		} else {
			amount := cursor.Count
			if packet.Button == 1 {
				amount = 1
			}
			MoveStack(slot, cursor, amount)
		}
	}
	c.Revision++
	s.syncContainer(session.Pos)
	s.sendContainerInventory(p.UUID)
}

func (s *Server) tickContainers() {
	for name, session := range s.ContainerSessions {
		s.ClientsMu.RLock()
		connected := s.Clients[name] != nil
		s.ClientsMu.RUnlock()
		if !connected || !s.containerReach(s.findPlayerEntity(name), session.Pos) {
			s.closeContainer(name)
		}
	}
	for pos, c := range s.Containers {
		if c.Kind != blockFurnace {
			continue
		}
		oldBurn, oldCook := c.Burn, c.Cook
		before := append([]ItemStack(nil), c.Slots...)
		c.tick()
		changed := false
		for i := range before {
			if before[i] != c.Slots[i] {
				changed = true
			}
		}
		if changed {
			c.Revision++
			s.syncContainer(pos)
		} else if (oldBurn != c.Burn || oldCook != c.Cook) && c.Burn%10 == 0 {
			s.syncContainer(pos)
		}
	}
}
func (s *Server) breakContainer(pos BlockPos) {
	c := s.Containers[pos]
	if c == nil {
		return
	}
	for name, session := range s.ContainerSessions {
		if session.Pos == pos {
			s.closeContainer(name)
		}
	}
	delete(s.Containers, pos)
	for i, stack := range c.Slots {
		if stack.ID != 0 {
			s.SpawnEntity(&ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("container-%d-%d-%d-%d-%d", pos.X, pos.Y, pos.Z, time.Now().UnixNano(), i), Type: EntityItem, X: float64(pos.X), Y: float64(pos.Y) + .3, Z: float64(pos.Z)}, ItemStack: stack, PickupDelay: .5, Vy: .1})
		}
	}
}

func (s *Server) saveContainers() error {
	data := struct {
		Version    int
		Containers []*BlockContainer
	}{Version: 1}
	for _, c := range s.Containers {
		data.Containers = append(data.Containers, c)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(s.SavePath, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.SavePath, "containers-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
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
	return os.Rename(f.Name(), filepath.Join(s.SavePath, "containers.json"))
}
func (s *Server) loadContainers() error {
	b, err := os.ReadFile(filepath.Join(s.SavePath, "containers.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var data struct {
		Version    int
		Containers []*BlockContainer
	}
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}
	if data.Version != 1 {
		return fmt.Errorf("unsupported container save")
	}
	loaded := make(map[BlockPos]*BlockContainer)
	for _, c := range data.Containers {
		if c == nil || containerSize(c.Kind) == 0 || len(c.Slots) != containerSize(c.Kind) || c.Pos.Y < 0 || c.Pos.Y >= chunkHeight || c.Burn < 0 || c.Burn > 16000 || c.BurnTotal < c.Burn || c.BurnTotal > 16000 || c.Cook < 0 || c.Cook >= 200 || loaded[c.Pos] != nil {
			return fmt.Errorf("invalid container save")
		}
		for i, stack := range c.Slots {
			if !validStack(stack) || (i != 2 && !acceptsContainerItem(c, i, stack)) {
				return fmt.Errorf("invalid container item")
			}
		}
		loaded[c.Pos] = c
	}
	s.Containers = loaded
	return nil
}

func (s *Server) sendContainerInventory(name string) {
	if p := s.findPlayerEntity(name); p != nil {
		s.SendInventory(p)
		s.SendTo(name, &PacketInventoryUpdate{SlotID: -1, ItemID: p.CursorItem.ID, Count: p.CursorItem.Count, Damage: p.CursorItem.Damage})
	}
}
