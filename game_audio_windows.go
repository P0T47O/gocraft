//go:build windows

package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	audioOnce     sync.Once
	audioEvents   chan gameSound
	playSoundProc = syscall.NewLazyDLL("winmm.dll").NewProc("PlaySoundW")
)

func playGameSound(kind gameSound) {
	audioOnce.Do(func() {
		audioEvents = make(chan gameSound, 4)
		go func() {
			for event := range audioEvents {
				wav := gameSoundWAV(event)
				// SND_MEMORY | SND_NODEFAULT; synchronous on the audio goroutine,
				// so the WAV remains alive until Windows finishes reading it.
				playSoundProc.Call(uintptr(unsafe.Pointer(&wav[0])), 0, 0x0004|0x0002)
				runtime.KeepAlive(wav)
			}
		}()
	})
	select {
	case audioEvents <- kind:
	default:
	}
}
