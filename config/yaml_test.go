// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromPathMigratesJSONToYAML(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, FileNameJSON)
	raw := []byte(`{"port":"5353","apiport":"8080","file_locations":{"dnsservers":"a.json","cache":"b.json","records_source":{"type":"file","location":"r.json"}},"DNSRecordSettings":{"auto_build_ptr_from_a":true,"forward_ptr_queries":false},"log":{"log_dir":"./l","log_severity":"none","log_rotation":"size","log_rotation_size_mb":100,"log_rotation_time_days":7}}`)
	if err := os.WriteFile(jsonPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(dir, FileName)
	if loaded.WritePath != yamlPath {
		t.Fatalf("WritePath = %q, want %q", loaded.WritePath, yamlPath)
	}
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("yaml not created: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal("json should remain after migration")
	}
	if loaded.Config.DNSPort != "5353" {
		t.Fatalf("DNSPort = %q", loaded.Config.DNSPort)
	}
}

func TestLoadPrefersYAMLWhenBothExist(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, FileNameJSON)
	yamlPath := filepath.Join(dir, FileName)
	_ = os.WriteFile(jsonPath, []byte(`{"port":"1111","apiport":"8080","file_locations":{"dnsservers":"a.json","cache":"b.json","records_source":{"type":"file","location":"r.json"}},"DNSRecordSettings":{"auto_build_ptr_from_a":true,"forward_ptr_queries":false},"log":{"log_dir":"./l","log_severity":"none","log_rotation":"size","log_rotation_size_mb":100,"log_rotation_time_days":7}}`), 0o600)
	_ = os.WriteFile(yamlPath, []byte("port: \"2222\"\napiport: \"8080\"\nfile_locations:\n  dnsservers: a.json\n  cache: b.json\n  records_source:\n    type: file\n    location: r.json\nDNSRecordSettings:\n  auto_build_ptr_from_a: true\n  forward_ptr_queries: false\nlog:\n  log_dir: ./l\n  log_severity: none\n  log_rotation: size\n  log_rotation_size_mb: 100\n  log_rotation_time_days: 7\n"), 0o600)

	loaded, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Config.DNSPort != "2222" {
		t.Fatalf("expected yaml port 2222, got %q", loaded.Config.DNSPort)
	}
}

func TestSaveWritesYAMLNotJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, FileNameJSON)
	yamlPath := filepath.Join(dir, FileName)
	_ = os.WriteFile(jsonPath, []byte(`{"port":"53","apiport":"8080","file_locations":{"dnsservers":"a.json","cache":"b.json","records_source":{"type":"file","location":"r.json"}},"DNSRecordSettings":{"auto_build_ptr_from_a":true,"forward_ptr_queries":false},"log":{"log_dir":"./l","log_severity":"none","log_rotation":"size","log_rotation_size_mb":100,"log_rotation_time_days":7}}`), 0o600)
	loaded, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Config.DNSPort = "9999"
	if err := Save(loaded.WritePath, loaded.Config); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig(yamlPath)
	if err != nil || cfg.DNSPort != "9999" {
		t.Fatalf("yaml save: port=%q err=%v", cfg.DNSPort, err)
	}
	cfg2, err := readConfig(jsonPath)
	if err != nil || cfg2.DNSPort != "53" {
		t.Fatalf("json should be untouched, port=%q", cfg2.DNSPort)
	}
}