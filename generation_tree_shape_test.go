package main

import "testing"

func TestGeneratedCrownEdgeVariationIsDeterministic(t *testing.T) {
	type pos struct{ x, y, z int }
	capture := func(a treeAnchor) map[pos]byte {
		blocks := make(map[pos]byte)
		a.emit(func(x, y, z int, block byte) { blocks[pos{x, y, z}] = block })
		return blocks
	}
	minRim, maxRim := 100, 0
	for i := 0; i < 24; i++ {
		a := treeAnchor{x: i * 7, y: 1, z: i * 11, height: 5, log: blockLog, leaves: blockLeaves, seed: 42}
		first, second := capture(a), capture(a)
		if len(first) != len(second) {
			t.Fatal("same tree anchor generated different block counts")
		}
		for p, block := range first {
			if second[p] != block {
				t.Fatalf("same tree anchor changed at %+v", p)
			}
		}
		if first[pos{a.x, a.y + a.height, a.z}] != blockLeaves {
			t.Fatal("crown lost its supported center")
		}
		rim := 0
		for p, block := range first {
			if block == blockLeaves && p.y == a.y+a.height-1 && (absInt(p.x-a.x) == 2 || absInt(p.z-a.z) == 2) {
				rim++
			}
		}
		minRim, maxRim = min(minRim, rim), max(maxRim, rim)
	}
	if minRim == maxRim {
		t.Fatalf("all canopies have the same outer edge (%d)", minRim)
	}
}
