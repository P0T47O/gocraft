package main

import (
	"io"
	"os"
	"path/filepath"
)

// writeSaveFile stages a complete file beside its destination before replacing
// it. A write, sync or close failure never truncates the previous save. This is
// per-file replacement, not a transaction across the world's separate files;
// rename atomicity and power-loss durability depend on the filesystem/platform.
func writeSaveFile(path string, data []byte) error {
	return replaceSaveFile(path, func(w io.Writer) error {
		n, err := w.Write(data)
		if err == nil && n != len(data) {
			return io.ErrShortWrite
		}
		return err
	})
}

func replaceSaveFile(path string, write func(io.Writer) error) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Do not remove the destination first: a failed replacement must retain it.
	return os.Rename(f.Name(), path)
}
