//go:build windows

package main

import "math"

// Conservative animation-independent sphere: summing all joint/offset lengths
// bounds every parent chain under arbitrary rotations, including death poses.
func (r *webGPUEntityRenderer) entityRadius(e *RemoteEntity) float32 {
	if e.MobKind == "" {
		return 2
	}
	if r.radii == nil {
		r.radii = make(map[string]float32)
	}
	if radius, ok := r.radii[e.MobKind]; ok {
		return radius
	}
	radius := float32(2)
	definition := mobContent.Definitions[e.MobKind]
	model := mobContent.Models[definition.Model]
	length := func(v [3]float32) float32 { return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]))) }
	for _, bone := range model.Bones {
		radius += length(bone.Pivot) + length(bone.Offset) + length(bone.Size)/2
	}
	r.radii[e.MobKind] = radius
	return radius
}
