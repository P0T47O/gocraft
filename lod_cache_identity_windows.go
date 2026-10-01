//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// Directory creation time survives saves but changes when a save directory is
// deleted and recreated with the same name/seed. No player save is modified.
func lodLocalCacheIdentity(path string) string {
	path = strings.ToLower(path)
	if info, err := os.Stat(path); err == nil {
		if attributes, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
			return fmt.Sprintf("%s:%d", path, attributes.CreationTime.Nanoseconds())
		}
	}
	return path
}
