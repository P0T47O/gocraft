package main

func drawDeathScreen(s *InputState) {
	menuRectangle(0, 0, int32(menuWidth()), int32(menuHeight()), meshColor(28, 13, 16, 210))
	l := survivalLayout(float32(menuWidth()), float32(menuHeight()))
	menuBox(l.Rect(220, 144, 560, 340), invBackground, invLine)
	menuRect(l.Rect(220, 144, 560, 3), invWarning)
	l.Text("YOU DIED", 252, 176, 32, invText)
	l.Text(s.Vitals.Cause, 252, 226, 18, invWarning)
	l.Text("Your items were dropped at the death location.", 252, 264, 14, invMuted)
	l.Text("Respawn restores health and air.", 252, 289, 14, invMuted)
	label := "Respawn"
	if s.RespawnWaiting {
		label = "Preparing a safe spawn..."
	}
	if ui.DrawAction(l.Rect(252, 338, 496, 42), label, !s.RespawnWaiting, true) {
		s.RespawnWaiting = true
		s.RespawnLast = 0
	}
	if ui.DrawButton(l.Rect(252, 394, 496, 38), "Return to menu", true) {
		exitGame()
	}
}
