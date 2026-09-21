//go:build !windows

package main

import "fmt"

func runNativeWebGPU(settings *GameSettings) error {
	return fmt.Errorf("native WebGPU window currently supports Windows only")
}
