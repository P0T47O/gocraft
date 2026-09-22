package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphicsFailureLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graphics-error.log")
	for _, reason := range []string{"device lost", "surface lost"} {
		message := graphicsFailureMessage(errors.New(reason), path)
		if !strings.Contains(message, reason) || !strings.Contains(message, "不代表存档成功") {
			t.Fatal(message)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "device lost") || !strings.Contains(string(data), "surface lost") {
		t.Fatalf("append log: %s %v", data, err)
	}
	message := graphicsFailureMessage(errors.New("device lost"), t.TempDir())
	if !strings.Contains(message, "日志写入失败") {
		t.Fatal("log error hidden")
	}
}
