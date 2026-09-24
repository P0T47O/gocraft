package main

import (
	"encoding/binary"
	"math"
)

type gameSound byte

const (
	soundEat gameSound = iota + 1
	soundHurt
)

// Small generated PCM effects avoid adding an audio asset/runtime dependency.
// Playback is platform-specific; dedicated servers never invoke it.
func gameSoundWAV(kind gameSound) []byte {
	const rate = 22050
	length := rate / 7
	if kind == soundHurt {
		length = rate / 5
	}
	pcmBytes := length * 2
	wav := make([]byte, 44+pcmBytes)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1) // PCM
	binary.LittleEndian.PutUint16(wav[22:], 1) // mono
	binary.LittleEndian.PutUint32(wav[24:], rate)
	binary.LittleEndian.PutUint32(wav[28:], rate*2)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(pcmBytes))
	var noise uint32 = 0x9e3779b9
	for i := 0; i < length; i++ {
		t := float64(i) / rate
		envelope := math.Pow(1-float64(i)/float64(length), 1.8)
		var sample float64
		switch kind {
		case soundEat:
			noise ^= noise << 13
			noise ^= noise >> 17
			noise ^= noise << 5
			crunch := float64(int32(noise)) / float64(math.MaxInt32)
			sample = (0.4*crunch + 0.6*math.Sin(2*math.Pi*(190+600*t)*t)) * envelope
		case soundHurt:
			sample = math.Sin(2*math.Pi*(220-120*t)*t) * envelope
		}
		binary.LittleEndian.PutUint16(wav[44+i*2:], uint16(int16(sample*5500)))
	}
	return wav
}
