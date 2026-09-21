package main

import "math"

// Pose evaluation is CPU-only. Both the active renderer and tests consume it;
// asset validation guarantees parents precede children in model.Bones.
type mobBonePose struct {
	joint, center [3]float32
	angleX        float32
}

func evaluateMobPose(model MobModel, animation MobAnimation, phase, blend float32, dst []mobBonePose) []mobBonePose {
	dst = dst[:0]
	angles := make(map[string]float32, len(animation.Channels))
	for _, channel := range animation.Channels {
		angles[channel.Bone] = float32(math.Sin(float64(phase+channel.Phase))) * animation.Amplitude * blend
	}
	joints := make(map[string]mobBonePose, len(model.Bones))
	for _, bone := range model.Bones {
		parent := joints[bone.Parent]
		joint := addVec3(parent.joint, rotateXVec(bone.Pivot, parent.angleX))
		angle := parent.angleX + angles[bone.Name]
		pose := mobBonePose{joint: joint, center: addVec3(joint, rotateXVec(bone.Offset, angle)), angleX: angle}
		joints[bone.Name] = pose
		dst = append(dst, pose)
	}
	return dst
}

func rotateXVec(v [3]float32, angle float32) [3]float32 {
	s, c := float32(math.Sin(float64(angle))), float32(math.Cos(float64(angle)))
	return [3]float32{v[0], v[1]*c - v[2]*s, v[1]*s + v[2]*c}
}

func addVec3(a, c [3]float32) [3]float32 {
	return [3]float32{a[0] + c[0], a[1] + c[1], a[2] + c[2]}
}
