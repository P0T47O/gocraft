package main

import (
	"math"
	"testing"
)

func TestMobPoseParentRotationAndOffset(t *testing.T) {
	model := MobModel{Bones: []MobBone{
		{Name: "root", Pivot: [3]float32{1, 2, 3}},
		{Name: "child", Parent: "root", Pivot: [3]float32{0, 1, 0}, Offset: [3]float32{0, 0, 2}},
	}}
	animation := MobAnimation{Amplitude: math.Pi / 2}
	animation.Channels = append(animation.Channels, struct {
		Bone  string
		Phase float32
	}{Bone: "root"})
	poses := evaluateMobPose(model, animation, math.Pi/2, 1, nil)
	want := [3]float32{1, 0, 4} // parent pivot + rotated child pivot + rotated offset
	for i, value := range poses[1].center {
		if math.Abs(float64(value-want[i])) > 1e-5 {
			t.Fatalf("child center = %v, want %v", poses[1].center, want)
		}
	}
	if math.Abs(float64(poses[1].angleX-math.Pi/2)) > 1e-5 {
		t.Fatal("child did not inherit parent rotation")
	}
	idle := evaluateMobPose(model, animation, 123, 0, poses[:0])
	if len(idle) != 2 || idle[1].center != ([3]float32{1, 3, 5}) {
		t.Fatalf("idle pose = %+v", idle)
	}
	if len(evaluateMobPose(MobModel{}, animation, 0, 1, idle)) != 0 {
		t.Fatal("reused output retained stale bones")
	}
}
