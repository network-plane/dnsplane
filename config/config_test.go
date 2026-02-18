package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromPathCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	loaded, err := LoadFromPath(dir)
	if err != nil {
		t.Fatalf("LoadFromPath(%q) error: %v", dir, err)
	}
	if !loaded.Created {
		t.Error("expected Created true when creating default config")
	}
	if loaded.Config.Log.Dir == "" {
		t.Error("default config should have non-empty Log.Dir")
	}
	if loaded.Config.ClientSocketPath == "" {
		t.Error("default config should have non-empty ClientSocketPath")
	}
	// Config file should exist
	if _, err := os.Stat(loaded.Path); err != nil {
		t.Errorf("config file not created at %q: %v", loaded.Path, err)
	}
	expectedPath := filepath.Join(dir, FileName)
	if loaded.Path != expectedPath {
		t.Errorf("loaded.Path = %q, want %q", loaded.Path, expectedPath)
	}
}
