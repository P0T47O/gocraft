package main

func (s *InputState) closeContainerUI() {
	if s.Container != nil {
		s.ClosedContainerToken = s.Container.Token
	}
	if s.Container != nil && client != nil {
		client.Send(&PacketContainerClick{Token: s.Container.Token, Slot: -1})
	}
	s.Container = nil
}
func (s *InputState) updateContainerInput() {
	view := s.Container
	if view == nil {
		return
	}
	l := survivalLayout(float32(windowWidth()), float32(windowHeight()))
	mouse := inputMousePosition()
	left, right := inputMousePressed(mouseLeft), inputMousePressed(mouseRight)
	if left && uiContainsPoint(mouse, l.Rect(938, 22, 38, 32)) {
		s.closeContainerUI()
		s.InventoryOpen = false
		s.SkipCamera = true
		captureCursor()
		return
	}
	if !left && !right {
		return
	}
	button := int32(0)
	if right {
		button = 1
	}
	if inputKeyDown(keyLeftShift) || inputKeyDown(keyRightShift) {
		button = 2
	}
	for i := 0; i < len(view.State.Slots)+36; i++ {
		if uiContainsPoint(mouse, containerUISlot(l, view.State.Kind, i)) && client != nil {
			client.Send(&PacketContainerClick{Token: view.Token, Revision: view.State.Revision, Slot: int32(i), Button: button})
			return
		}
	}
}
