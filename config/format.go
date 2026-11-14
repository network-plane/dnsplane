// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dnsplane/internal/atomicfile"

	"gopkg.in/yaml.v3"
)

func decodeConfigBytes(path string, data []byte) (*Config, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("config: file %s is empty", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	jsonData := data
	if ext == ".yaml" || ext == ".yml" {
		var v any
		if err := yaml.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
		var err error
		jsonData, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
	}

	var cfg Config
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if legacy := extractLegacyRecordSettings(jsonData); legacy != nil {
		cfg.DNSRecordSettings = *legacy
	}
	return &cfg, nil
}

func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path from operator (config file)
	if err != nil {
		return nil, err
	}
	return decodeConfigBytes(path, data)
}

func writeConfig(path string, cfg *Config) error {
	ext := strings.ToLower(filepath.Ext(path))
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("config: ensure config directory %s: %w", dir, err)
	}
	cfg.applyDefaults(dir)
	var payload []byte
	var err error
	switch ext {
	case ".yaml", ".yml":
		j, mErr := json.Marshal(cfg)
		if mErr != nil {
			return fmt.Errorf("config: marshal config: %w", mErr)
		}
		var v any
		if err = json.Unmarshal(j, &v); err != nil {
			return fmt.Errorf("config: marshal config: %w", err)
		}
		payload, err = yaml.Marshal(v)
	default:
		payload, err = json.MarshalIndent(cfg, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("config: marshal config: %w", err)
	}
	if err := atomicfile.Write(path, payload, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}