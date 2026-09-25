package main

import (
	"bytes"
	"math"
	"testing"
)

func TestDaylightCycleAndMidnightContinuity(t *testing.T) {
	noon, night := worldDaylight(6000), worldDaylight(18000)
	if noon.Brightness != 1 || night.Brightness >= .35 || night.Sky[2] >= noon.Sky[2] {
		t.Fatalf("day/night palette is not distinct: day=%+v night=%+v", noon, night)
	}
	for _, tick := range []float64{0, 1000, 6000, 12000, 18000, 23999} {
		a, b := worldDaylight(tick), worldDaylight(tick+dayLengthTicks)
		if a != b {
			t.Fatalf("cycle is not periodic at %.0f: %+v != %+v", tick, a, b)
		}
	}
	before, after := worldDaylight(23999.9), worldDaylight(.1)
	if math.Abs(float64(before.Brightness-after.Brightness)) > .001 {
		t.Fatalf("midnight brightness jumped: %v -> %v", before, after)
	}
}

func TestWorldTimePacketRoundTrip(t *testing.T) {
	var wire bytes.Buffer
	if err := WritePacket(&wire, &PacketWorldTime{Ticks: 24000*123 + 6000}); err != nil {
		t.Fatal(err)
	}
	pkt, err := ReadPacket(&wire)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := pkt.(*PacketWorldTime)
	if !ok || got.Ticks != 24000*123+6000 {
		t.Fatalf("clock packet changed: %T %+v", pkt, pkt)
	}
	if err := (&PacketWorldTime{}).Decode(bytes.NewBuffer([]byte{1, 2, 3})); err == nil {
		t.Fatal("truncated clock packet accepted")
	}
}

func TestNightRetainsBlockLight(t *testing.T) {
	if got := lightRetention(15, 0, 0); got > 27 || got < 24 {
		t.Fatalf("sunlit surface retained too much light: %d", got)
	}
	if got := lightRetention(15, 15, 0); got != 255 {
		t.Fatalf("torch-lit surface lost block light: %d", got)
	}
	if got := lightRetention(0, 0, 0); got != 255 {
		t.Fatalf("unlit cave should retain its existing ambient floor: %d", got)
	}
}
