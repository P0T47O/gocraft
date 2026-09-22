//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func reportNativeGraphicsFailure(err error) {
	path, pathErr := filepath.Abs("graphics-error.log")
	if pathErr != nil {
		path = "graphics-error.log"
	}
	// Remove embedded NULs so an arbitrary driver error cannot hide the message.
	message := strings.ReplaceAll(graphicsFailureMessage(err, path), "\x00", "�")
	body, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("GoCraft 图形错误")
	nativeUserProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), 0x10)
}
