package main

type explosionEffect struct {
	x, y, z float32
	radius  float32
	age     float32
}

var explosionEffects []explosionEffect

func addExplosionEffect(p *PacketExplosion) {
	if p.Radius <= 0 || p.Radius > 6 {
		return
	}
	if len(explosionEffects) >= 32 {
		explosionEffects = explosionEffects[1:]
	}
	explosionEffects = append(explosionEffects, explosionEffect{float32(p.X), float32(p.Y), float32(p.Z), p.Radius, 0})
}

func tickExplosionEffects(dt float32) {
	kept := explosionEffects[:0]
	for _, effect := range explosionEffects {
		effect.age += dt
		if effect.age < .65 {
			kept = append(kept, effect)
		}
	}
	explosionEffects = kept
}
