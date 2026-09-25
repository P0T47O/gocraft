package main

// All drawing and hit testing use this same layout, including window resizing.
type SurvivalLayout struct{ X, Y, S float32 }

func survivalLayout(w, h float32) SurvivalLayout {
	s := min((w-32)/1000, (h-32)/620, float32(1.35))
	return SurvivalLayout{(w - 1000*s) / 2, (h - 620*s) / 2, s}
}
func (l SurvivalLayout) Rect(x, y, w, h float32) uiRect {
	return newUIRect(l.X+x*l.S, l.Y+y*l.S, w*l.S, h*l.S)
}
func (l SurvivalLayout) Slot(i int) uiRect {
	if i >= armorSlotStart && i < armorSlotStart+armorSlotCount {
		return l.Rect(24+float32(i-armorSlotStart)*70, 558, 54, 54)
	}
	if i < 9 {
		return l.Rect(354+float32(i)*68, 516, 60, 60)
	}
	return l.Rect(354+float32((i-9)%9)*68, 294+float32((i-9)/9)*64, 60, 60)
}
func (l SurvivalLayout) Row(i int) uiRect { return l.Rect(24, 144+float32(i)*64, 284, 58) }

func containerUISlot(l SurvivalLayout, kind byte, i int) uiRect {
	n := containerSize(kind)
	if i < n {
		if kind == blockChest {
			return l.Rect(198+float32(i%9)*66, 70+float32(i/9)*62, 58, 56)
		}
		switch i {
		case 0:
			return l.Rect(330, 80, 64, 64)
		case 1:
			return l.Rect(330, 170, 64, 64)
		default:
			return l.Rect(600, 120, 76, 76)
		}
	}
	i -= n
	if i < 9 {
		return l.Rect(198+float32(i)*66, 516, 58, 56)
	}
	return l.Rect(198+float32((i-9)%9)*66, 310+float32((i-9)/9)*62, 58, 56)
}
