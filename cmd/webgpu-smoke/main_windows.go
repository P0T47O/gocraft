//go:build windows

// The smoke command is a source-tree development launcher for the native
// real-chunk probe. It no longer maintains a second graphics/window stack.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
		if readErr == nil && strings.HasPrefix(string(data), "module gocraft") {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "Run this development command inside the GoCraft source tree.")
			os.Exit(1)
		}
		dir = parent
	}
	cmd := exec.Command("go", "test", ".", "-run", "^TestWebGPUChunkPreview$", "-count=1", "-v", "-timeout=0")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOCRAFT_WEBGPU_CHUNK_PREVIEW=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOCRAFT_WEBGPU_CHUNK_PREVIEW=1")
	if err = cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
