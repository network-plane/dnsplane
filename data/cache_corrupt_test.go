// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"os"
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestLoadCacheRecordsCorruptStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "dnscache.json")
	if err := os.WriteFile(cachePath, []byte(`{"cache":`), 0o600); err != nil {
		t.Fatal(err)
	}
	SetConfig(&config.Loaded{
		Path:      filepath.Join(dir, "dnsplane.yaml"),
		WritePath: filepath.Join(dir, "dnsplane.yaml"),
		Config: config.Config{
			FileLocations: config.FileLocations{
				DNSServerFile: filepath.Join(dir, "dnsservers.json"),
				CacheFile:     cachePath,
				RecordsSource: &config.RecordsSourceConfig{Type: config.RecordsSourceFile, Location: filepath.Join(dir, "dnsrecords.json")},
			},
		},
	})
	rec, err := LoadCacheRecords()
	if err != nil {
		t.Fatalf("corrupt cache should not error: %v", err)
	}
	if rec != nil && len(rec) != 0 {
		t.Fatalf("expected nil or empty cache, got %d", len(rec))
	}
}