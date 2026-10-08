package main

import (
	"math"
	"testing"
)

func TestPassiveMobFacesVisibleMovement(t *testing.T) {
	previous := remoteEntities
	t.Cleanup(func() { remoteEntities = previous })
	for _, kind := range []string{"pig", "sheep", "chicken"} {
		t.Run(kind, func(t *testing.T) {
			mob := &RemoteEntity{MobKind: kind, MobState: "walk", X: 0, Z: 0, TX: 1, TZ: 0, Yaw: -math.Pi / 2, TargetYaw: -math.Pi / 2}
			remoteEntities = map[string]*RemoteEntity{kind: mob}
			updateInterpolation(.05)
			if mob.X <= 0 || math.Cos(float64(mob.Yaw-math.Pi/2)) < .99 {
				t.Fatalf("%s moved east while facing %f radians", kind, mob.Yaw)
			}
			// Reversing the target position must reverse the visible facing,
			// even while the network's old facing has not caught up.
			mob.TX = -1
			updateInterpolation(.05)
			if math.Cos(float64(mob.Yaw+math.Pi/2)) < .99 {
				t.Fatalf("%s moved west while facing %f radians", kind, mob.Yaw)
			}
		})
	}
}

func TestMobIdleAndHostileFacingRemainAuthoritative(t *testing.T) {
	previous := remoteEntities
	t.Cleanup(func() { remoteEntities = previous })
	idle := &RemoteEntity{MobKind: "pig", MobState: "idle", TargetYaw: math.Pi / 2}
	hostile := &RemoteEntity{MobKind: "zombie", MobState: "chase", TX: 1, TargetYaw: -math.Pi / 2}
	remoteEntities = map[string]*RemoteEntity{"idle": idle, "hostile": hostile}
	updateInterpolation(.05)
	if idle.Yaw <= 0 || idle.Yaw >= math.Pi/2 {
		t.Fatalf("idle pig did not turn toward server yaw: %f", idle.Yaw)
	}
	if hostile.X <= 0 || hostile.Yaw >= 0 {
		t.Fatalf("chasing zombie lost authoritative aim: x=%f yaw=%f", hostile.X, hostile.Yaw)
	}
}
