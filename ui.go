package main

func uiScaleFor(w, h float32) float32 {
	scale := float32(1)
	if w > 0 && h > 0 {
		scale = w / 1280
		if h/720 < scale {
			scale = h / 720
		}
	}
	if scale < 1 {
		scale = 1
	}
	return scale
}

func inventoryScaleFor(w, h float32) float32 {
	return min(uiScaleFor(w, h)*3.2, (w-32)/176, (h-32)/196)
}

type InventoryLayout struct {
	OriginX  float32
	OriginY  float32
	SlotSize float32
	Stride   float32
	Cols     int
	Rows     int
	GridX    float32
	GridY    float32
	GridW    float32 // Width of the grid area
	GridH    float32 // Height of the grid area
	HotbarX  float32
	HotbarY  float32
}

func inventoryLayoutFor(w, h float32) InventoryLayout {
	scale := inventoryScaleFor(w, h)
	// Creative layout: 9 columns, 6 visible rows.
	texW := float32(176) * scale
	texH := float32(196) * scale
	originX := w/2 - texW/2
	originY := h/2 - texH/2
	slot := float32(18) * scale
	stride := slot
	gridW := float32(9) * stride

	gridX := originX + (texW-gridW)/2
	gridY := originY + float32(18)*scale // Top padding

	hotbarX := gridX
	hotbarY := originY + texH - float32(24)*scale

	return InventoryLayout{
		OriginX:  originX,
		OriginY:  originY,
		SlotSize: slot,
		Stride:   stride,
		Cols:     9,
		Rows:     6,
		GridX:    gridX,
		GridY:    gridY,
		GridW:    gridW,
		GridH:    float32(6) * stride,
		HotbarX:  hotbarX,
		HotbarY:  hotbarY,
	}
}

func (l InventoryLayout) creativeTotalRows(itemCount int) int {
	if l.Cols <= 0 || itemCount <= 0 {
		return 0
	}
	return (itemCount + l.Cols - 1) / l.Cols
}

func (l InventoryLayout) creativeMaxScroll(itemCount int) int {
	return max(0, l.creativeTotalRows(itemCount)-l.Rows)
}

func (l InventoryLayout) creativeScroll(scroll, itemCount int) int {
	return max(0, min(scroll, l.creativeMaxScroll(itemCount)))
}

func (l InventoryLayout) creativeIndex(scroll, row, col, itemCount int) int {
	return (l.creativeScroll(scroll, itemCount)+row)*l.Cols + col
}
