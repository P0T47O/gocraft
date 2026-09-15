//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("The first GoCraft WebGPU smoke probe currently targets Windows because it reuses Raylib's HWND. Other surface adapters will follow after the renderer bring-up is proven.")
}
