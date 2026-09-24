package main

import (
	"encoding/binary"
	"testing"
)

func TestGameSoundWAV(t *testing.T) {
	for _, kind := range []gameSound{soundEat, soundHurt} {
		wav := gameSoundWAV(kind)
		if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:16]) != "WAVEfmt " || string(wav[36:40]) != "data" {
			t.Fatalf("invalid WAV header for sound %d", kind)
		}
		if int(binary.LittleEndian.Uint32(wav[4:8])) != len(wav)-8 || int(binary.LittleEndian.Uint32(wav[40:44])) != len(wav)-44 {
			t.Fatalf("invalid WAV lengths for sound %d", kind)
		}
	}
}
