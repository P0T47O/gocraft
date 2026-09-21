//go:build !windows

package main

import "fmt"

func runMobPreview() error { return fmt.Errorf("native mob preview currently supports Windows only") }
