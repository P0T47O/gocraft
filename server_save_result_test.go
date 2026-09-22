package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerSaveReturnsFailures(t *testing.T) {
	w := NewClientWorld()
	defer w.Close()
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{World: w, SavePath: path}
	err := s.Save()
	if err == nil || !strings.Contains(err.Error(), "Container") || !strings.Contains(err.Error(), "Player") {
		t.Fatalf("missing aggregated errors: %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "keep" {
		t.Fatal("existing file changed")
	}
	logPath := filepath.Join(t.TempDir(), "save-error.log")
	message := saveFailureMessage(err, logPath)
	data, readErr = os.ReadFile(logPath)
	if readErr != nil || !strings.Contains(string(data), err.Error()) || !strings.Contains(message, "World save failed") {
		t.Fatal("failure log missing")
	}
	s.SavePath = t.TempDir()
	if err := s.Save(); err != nil {
		t.Fatalf("valid save: %v", err)
	}
}
