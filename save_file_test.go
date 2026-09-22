package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFileReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.bin")
	for _, data := range []string{"original save", "new", ""} {
		if err := writeSaveFile(path, []byte(data)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != data {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
}

func TestSaveFileFailurePreservesPrevious(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.bin")
	if err := writeSaveFile(path, []byte("previous")); err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected write failure")
	err := replaceSaveFile(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("partial")); err != nil {
			return err
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "previous" {
			t.Fatalf("destination modified before commit: %q, %v", got, err)
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "previous" {
		t.Fatalf("previous save lost: %q, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("failed write leaked temporary file")
	}
}

func TestSaveFileRenameFailureCleansTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "occupied")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeSaveFile(filepath.Join(path, "keep"), []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := writeSaveFile(path, []byte("new")); err == nil {
		t.Fatal("expected replacement failure")
	}
	if b, err := os.ReadFile(filepath.Join(path, "keep")); err != nil || string(b) != "keep" {
		t.Fatal("destination changed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("rename failure leaked temporary file")
	}
}

func TestChunkSaveRoundTripAndDirtyFailure(t *testing.T) {
	dir := t.TempDir()
	c := &Chunk{}
	c.blocks.Set(0, 0, 0, blockStone)
	c.blocks.Set(15, 255, 15, blockTorch)
	c.meta.Set(15, 255, 15, 4)
	for _, id := range []byte{blockStone, blockGrass} {
		c.blocks.Set(3, 20, 7, id)
		c.dirty = true
		if err := SaveChunk(dir, c, -2, 3); err != nil {
			t.Fatal(err)
		}
		if c.dirty {
			t.Fatal("successful save left chunk dirty")
		}
		loaded := &Chunk{}
		if err := loadChunkFile(dir, -2, 0, 3, &loaded.blocks, &loaded.meta); err != nil {
			t.Fatal(err)
		}
		if !loaded.blocks.Equal(&c.blocks) || !loaded.meta.Equal(&c.meta) {
			t.Fatal("round trip lost block/metadata")
		}
	}

	missing := &Chunk{}
	if TryLoadChunk(dir, missing, 99, 99) {
		t.Fatal("missing chunk should fall back to generation")
	}

	corruptPath := filepath.Join(dir, chunkDir, "7_0_9.bin")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corruptPath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("corrupt chunk save was treated as missing")
			}
		}()
		_ = TryLoadChunk(dir, &Chunk{}, 7, 9)
	}()
	if got, err := os.ReadFile(corruptPath); err != nil || string(got) != "corrupt" {
		t.Fatalf("corrupt chunk save was modified: %q, %v", got, err)
	}

	c.dirty = true
	if err := SaveChunk(filepath.Join(dir, chunkDir, "-2_0_3.bin"), c, -2, 3); err == nil {
		t.Fatal("expected save failure")
	}
	if !c.dirty {
		t.Fatal("failed save cleared dirty flag")
	}
}

func TestServerRetainsChunkWhenUnloadSaveFails(t *testing.T) {
	initBlockRegistry()
	w := NewFlatWorld()
	defer w.Close()
	key := chunkKey{X: 100, Z: 100}
	c := lifecycleChunk(w, key)
	c.blocks.Set(0, 1, 0, blockStone)
	c.dirty = true
	root := filepath.Join(t.TempDir(), "not-a-directory")
	if err := writeSaveFile(root, []byte("occupied")); err != nil {
		t.Fatal(err)
	}
	client := &ClientConnection{KnownChunks: map[chunkKey]bool{key: true}, Send: make(chan Packet, 8), done: make(chan struct{})}
	s := &Server{World: w, SavePath: root, Clients: map[string]*ClientConnection{"test": client}, PendingChunks: make(map[chunkKey][]string)}
	s.Tick()
	if w.chunks[key] != c || !c.dirty || c.blocks.Get(0, 1, 0) != blockStone || !client.KnownChunks[key] {
		t.Fatal("failed save discarded resident data/client state")
	}
	s.SavePath = t.TempDir()
	s.Tick()
	if w.chunks[key] != nil || client.KnownChunks[key] {
		t.Fatal("successful retry did not unload chunk")
	}
	loaded := &Chunk{}
	if err := loadChunkFile(s.SavePath, key.X, 0, key.Z, &loaded.blocks, &loaded.meta); err != nil {
		t.Fatal(err)
	}
	if loaded.blocks.Get(0, 1, 0) != blockStone {
		t.Fatal("retry lost edits")
	}
}
