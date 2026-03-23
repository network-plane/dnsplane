// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package config

import (
	"encoding/json"
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

// Legacy configs without cache_warm_* keys must keep documented defaults (warm on, 10s interval).
func TestUnmarshalJSON_CacheWarmLegacyDefaults(t *testing.T) {
	raw := []byte(`{"port":"53","apiport":"8080","file_locations":{"dnsservers":"a.json","cache":"b.json","records_source":{"type":"file","location":"r.json"}},"DNSRecordSettings":{"auto_build_ptr_from_a":true,"forward_ptr_queries":false},"log":{"log_dir":"./l","log_severity":"none","log_rotation":"size","log_rotation_size_mb":100,"log_rotation_time_days":7}}`)
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	if !c.CacheWarmEnabled {
		t.Fatal("cache_warm_enabled absent: want default true")
	}
	if c.CacheWarmIntervalSeconds != 10 {
		t.Fatalf("cache_warm_interval_seconds absent: want 10, got %d", c.CacheWarmIntervalSeconds)
	}
}

func TestApplyDefaults_PprofListenWhenEnabled(t *testing.T) {
	c := &Config{PprofEnabled: true, PprofListen: ""}
	c.applyDefaults(t.TempDir())
	if c.PprofListen != "127.0.0.1:6060" {
		t.Fatalf("PprofListen = %q, want 127.0.0.1:6060", c.PprofListen)
	}
}
