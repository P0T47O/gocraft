//go:build !windows

package main

import "fmt"

func main() { fmt.Println("The native WebGPU preview currently supports Windows only.") }
