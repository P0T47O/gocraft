//go:build !windows

package main

import "fmt"

func ensureExperimentalWebGPURenderer() error {
	return fmt.Errorf("experimental WebGPU game renderer currently supports Windows only")
}

func drawExperimentalWebGPUFrame() error {
	return fmt.Errorf("experimental WebGPU game renderer currently supports Windows only")
}

func closeExperimentalWebGPURenderer() {}
func releaseExperimentalWebGPULOD()    {}
