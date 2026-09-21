package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Temporary legacy renderer boundary. Gameplay and saves never use this type.
func raylibCamera(c gameCamera) rl.Camera3D {
	return rl.Camera3D{
		Position:   raylibVec3FromGame(c.Position),
		Target:     raylibVec3FromGame(c.Target),
		Up:         raylibVec3FromGame(c.Up),
		Fovy:       c.Fovy,
		Projection: rl.CameraPerspective,
	}
}

func raylibVec3FromGame(v gameVec3) rl.Vector3 { return rl.NewVector3(v.X, v.Y, v.Z) }
