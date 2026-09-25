package main

// ItemDef is shared, immutable type data. ItemStack owns per-instance state.
type ItemDef struct {
	ID                      byte
	Name, Icon              string
	MaxStack, MaxDurability int32
	PlaceBlock              byte
	ToolType                ToolType
	ToolMaterial            ToolMaterial
}

var Items [256]*ItemDef
var itemVisuals [256]BlockDef
var emptyItemDef = ItemDef{Name: "Unknown", MaxStack: 64}

func GetItem(id byte) *ItemDef {
	if d := Items[id]; d != nil {
		return d
	}
	return &emptyItemDef
}

func initBlockItems() {
	Items = [256]*ItemDef{}
	for _, d := range Blocks {
		if d != nil && d.ID != 0 {
			Items[d.ID] = &ItemDef{ID: d.ID, Name: d.Name, MaxStack: 64, PlaceBlock: d.ID}
		}
	}
}

func configureTools() {
	for _, d := range Items {
		if d == nil || d.ToolType == ToolNone {
			continue
		}
		d.MaxStack = 1
		switch d.ToolMaterial {
		case MatWood:
			d.MaxDurability = 59
		case MatStone:
			d.MaxDurability = 131
		case MatIron:
			d.MaxDurability = 250
		case MatDiamond:
			d.MaxDurability = 1561
		case MatGold:
			d.MaxDurability = 32
		}
	}
}

// Renderer adapter: physical blocks retain their faces; standalone items use sprites.
func GetItemVisual(id byte) *BlockDef {
	if Items[id] == nil {
		return GetBlock(blockAir)
	}
	d := GetItem(id)
	if d.PlaceBlock != 0 {
		return GetBlock(d.PlaceBlock)
	}
	return &itemVisuals[id]
}

// MoveStack copies all instance state when moving into an empty slot.
func MoveStack(dst, src *ItemStack, amount int32) int32 {
	if dst == src || src.ID == 0 || amount <= 0 || (dst.ID != 0 && !CanStack(*dst, *src)) {
		return 0
	}
	n := min(amount, src.Count, StackLimit(src.ID)-dst.Count)
	if n <= 0 {
		return 0
	}
	if dst.ID == 0 {
		*dst = *src
		dst.Count = 0
	}
	dst.Count += n
	src.Count -= n
	if src.Count == 0 {
		*src = ItemStack{}
	}
	return n
}

func StackLimit(id int32) int32 {
	if id <= 0 || id > 255 {
		return 0
	}
	return GetItem(byte(id)).MaxStack
}

func CanStack(a, b ItemStack) bool {
	return a.ID != 0 && a.ID == b.ID && a.Damage == b.Damage && StackLimit(a.ID) > 1
}

func (s *ItemStack) Wear(amount int32) {
	if s.ID <= 0 || s.ID > 255 || amount <= 0 {
		return
	}
	limit := GetItem(byte(s.ID)).MaxDurability
	if limit == 0 {
		return
	}
	s.Damage += amount
	if s.Damage >= limit {
		*s = ItemStack{}
	}
}

func validStack(s ItemStack) bool {
	if s.ID == 0 {
		return s.Count == 0 && s.Damage == 0
	}
	if s.ID < 0 || s.ID > 255 || Items[s.ID] == nil || s.Count < 1 || s.Count > StackLimit(s.ID) || s.Damage < 0 {
		return false
	}
	d := GetItem(byte(s.ID))
	return (d.MaxDurability == 0 && s.Damage == 0) || (d.MaxDurability > 0 && s.Damage < d.MaxDurability)
}

// Migration overflow stays in the player save until inventory space is available.
func (p *PlayerEntity) claimPendingItems() {
	if p.dead() {
		return
	}
	pending := p.PendingItems
	p.PendingItems = nil
	for _, stack := range pending {
		stack.Count = p.Inventory.AddStack(stack)
		if stack.Count > 0 {
			p.PendingItems = append(p.PendingItems, stack)
		}
	}
}

func migratePlayerItems(p *PlayerEntity) {
	for i := range p.Inventory.Slots {
		s := &p.Inventory.Slots[i]
		limit := StackLimit(s.ID)
		if s.ID != 0 && s.Count > limit {
			extra := *s
			extra.Count -= limit
			s.Count = limit
			p.PendingItems = append(p.PendingItems, extra)
		}
	}
	if s := &p.CursorItem; s.ID != 0 && s.Count > StackLimit(s.ID) {
		extra := *s
		extra.Count -= StackLimit(s.ID)
		s.Count = StackLimit(s.ID)
		p.PendingItems = append(p.PendingItems, extra)
	}
	p.claimPendingItems()
}

func initItemRegistry() {
	initBlockItems()
	// Tool definitions.
	registerTool := func(id byte, name string, tex string) {
		Items[id] = &ItemDef{ID: id, Name: name, Icon: tex, MaxStack: 64}
	}

	registerTool(itemWoodPickaxe, "Wooden Pickaxe", "textures/item/wooden_pickaxe.png")
	Items[itemWoodPickaxe].ToolType = ToolPickaxe
	Items[itemWoodPickaxe].ToolMaterial = MatWood

	registerTool(itemStonePickaxe, "Stone Pickaxe", "textures/item/stone_pickaxe.png")
	Items[itemStonePickaxe].ToolType = ToolPickaxe
	Items[itemStonePickaxe].ToolMaterial = MatStone

	registerTool(itemIronPickaxe, "Iron Pickaxe", "textures/item/iron_pickaxe.png")
	Items[itemIronPickaxe].ToolType = ToolPickaxe
	Items[itemIronPickaxe].ToolMaterial = MatIron

	registerTool(itemDiamondPickaxe, "Diamond Pickaxe", "textures/item/diamond_pickaxe.png")
	Items[itemDiamondPickaxe].ToolType = ToolPickaxe
	Items[itemDiamondPickaxe].ToolMaterial = MatDiamond

	registerTool(itemGoldPickaxe, "Gold Pickaxe", "textures/item/golden_pickaxe.png")
	Items[itemGoldPickaxe].ToolType = ToolPickaxe
	Items[itemGoldPickaxe].ToolMaterial = MatGold

	registerTool(itemWoodShovel, "Wooden Shovel", "textures/item/wooden_shovel.png")
	Items[itemWoodShovel].ToolType = ToolShovel
	Items[itemWoodShovel].ToolMaterial = MatWood

	registerTool(itemStoneShovel, "Stone Shovel", "textures/item/stone_shovel.png")
	Items[itemStoneShovel].ToolType = ToolShovel
	Items[itemStoneShovel].ToolMaterial = MatStone

	registerTool(itemIronShovel, "Iron Shovel", "textures/item/iron_shovel.png")
	Items[itemIronShovel].ToolType = ToolShovel
	Items[itemIronShovel].ToolMaterial = MatIron

	registerTool(itemDiamondShovel, "Diamond Shovel", "textures/item/diamond_shovel.png")
	Items[itemDiamondShovel].ToolType = ToolShovel
	Items[itemDiamondShovel].ToolMaterial = MatDiamond

	registerTool(itemGoldShovel, "Gold Shovel", "textures/item/golden_shovel.png")
	Items[itemGoldShovel].ToolType = ToolShovel
	Items[itemGoldShovel].ToolMaterial = MatGold

	registerTool(itemWoodAxe, "Wooden Axe", "textures/item/wooden_axe.png")
	Items[itemWoodAxe].ToolType = ToolAxe
	Items[itemWoodAxe].ToolMaterial = MatWood

	registerTool(itemStoneAxe, "Stone Axe", "textures/item/stone_axe.png")
	Items[itemStoneAxe].ToolType = ToolAxe
	Items[itemStoneAxe].ToolMaterial = MatStone

	registerTool(itemIronAxe, "Iron Axe", "textures/item/iron_axe.png")
	Items[itemIronAxe].ToolType = ToolAxe
	Items[itemIronAxe].ToolMaterial = MatIron

	registerTool(itemDiamondAxe, "Diamond Axe", "textures/item/diamond_axe.png")
	Items[itemDiamondAxe].ToolType = ToolAxe
	Items[itemDiamondAxe].ToolMaterial = MatDiamond

	registerTool(itemGoldAxe, "Gold Axe", "textures/item/golden_axe.png")
	Items[itemGoldAxe].ToolType = ToolAxe
	Items[itemGoldAxe].ToolMaterial = MatGold
	for _, hoe := range []struct {
		id            byte
		name, texture string
		material      ToolMaterial
	}{
		{itemWoodHoe, "Wooden Hoe", "textures/item/wooden_hoe.png", MatWood},
		{itemStoneHoe, "Stone Hoe", "textures/item/stone_hoe.png", MatStone},
		{itemIronHoe, "Iron Hoe", "textures/item/iron_hoe.png", MatIron},
		{itemDiamondHoe, "Diamond Hoe", "textures/item/diamond_hoe.png", MatDiamond},
		{itemGoldHoe, "Golden Hoe", "textures/item/golden_hoe.png", MatGold},
	} {
		registerTool(hoe.id, hoe.name, hoe.texture)
		Items[hoe.id].ToolType, Items[hoe.id].ToolMaterial = ToolHoe, hoe.material
	}

	// Standalone resource items.
	registerItem := func(id byte, name string, tex string) {
		Items[id] = &ItemDef{ID: id, Name: name, Icon: tex, MaxStack: 64}
	}

	registerItem(itemCoal, "Coal", "textures/item/coal.png")
	registerItem(itemIronIngot, "Iron Ingot", "textures/item/iron_ingot.png")
	registerItem(itemGoldIngot, "Gold Ingot", "textures/item/gold_ingot.png")
	registerItem(itemDiamond, "Diamond", "textures/item/diamond.png")
	registerItem(itemStick, "Stick", "textures/item/stick.png")
	registerItem(itemRawPork, "Raw Porkchop", "textures/item/porkchop.png")
	registerItem(itemCookedPork, "Cooked Porkchop", "textures/item/cooked_porkchop.png")
	registerItem(itemRottenFlesh, "Rotten Flesh", "textures/item/rotten_flesh.png")
	registerItem(itemBone, "Bone", "textures/item/bone.png")
	registerItem(itemArrow, "Arrow", "textures/item/arrow.png")
	registerItem(itemString, "String", "textures/item/string.png")
	registerItem(itemGunpowder, "Gunpowder", "textures/item/gunpowder.png")
	registerItem(itemWheatSeeds, "Wheat Seeds", "textures/item/wheat_seeds.png")
	registerItem(itemWheat, "Wheat", "textures/item/wheat.png")
	registerItem(itemBread, "Bread", "textures/item/bread.png")
	registerItem(itemPotato, "Potato", "textures/item/potato.png")
	registerItem(itemCarrot, "Carrot", "textures/item/carrot.png")
	registerItem(itemBakedPotato, "Baked Potato", "textures/item/baked_potato.png")
	configureTools()
	for id, d := range Items {
		if d != nil && d.PlaceBlock == 0 {
			itemVisuals[id] = BlockDef{ID: d.ID, Name: d.Name, Textures: blockFaces{North: d.Icon}, RenderType: RenderTypeCross, IsTransparent: true}
		}
	}
}
