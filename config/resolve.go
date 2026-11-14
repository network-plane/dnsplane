// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configPathsInDir(dir string) (yaml, yml, json string) {
	dir = filepath.Clean(dir)
	return filepath.Join(dir, FileName), filepath.Join(dir, FileNameYML), filepath.Join(dir, FileNameJSON)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func nagf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "dnsplane: "+format+"\n", args...)
}

// resolveLoadedConfig picks read/write paths in dir, handles JSON→YAML migration and startup nags.
// userPath is the path the operator passed (--config) or the candidate file path from search; empty when probing a dir only.
func resolveLoadedConfig(dir, userPath string) (*Loaded, error) {
	dir = filepath.Clean(dir)
	yamlPath, ymlPath, jsonPath := configPathsInDir(dir)
	writePath := yamlPath

	yamlExists := fileExists(yamlPath)
	ymlExists := fileExists(ymlPath)
	jsonExists := fileExists(jsonPath)
	readYAML := ""
	if yamlExists {
		readYAML = yamlPath
	} else if ymlExists {
		readYAML = ymlPath
	}

	userPath = strings.TrimSpace(userPath)
	userJSON := strings.HasSuffix(strings.ToLower(userPath), ".json")

	if readYAML != "" {
		if jsonExists {
			nagf("config: using %s; leftover %s is ignored (remove it or pass --config %s)", readYAML, jsonPath, readYAML)
		}
		if userJSON && filepath.Clean(userPath) == jsonPath {
			nagf("config: --config points at %s; using %s. Pass --config %s", jsonPath, readYAML, readYAML)
		}
		cfg, err := readConfig(readYAML)
		if err != nil {
			return nil, fmt.Errorf("config: failed to read %s: %w", readYAML, err)
		}
		cfg.applyDefaults(dir)
		return &Loaded{Path: readYAML, WritePath: writePath, Config: *cfg}, nil
	}

	if jsonExists {
		cfg, err := readConfig(jsonPath)
		if err != nil {
			return nil, fmt.Errorf("config: failed to read %s: %w", jsonPath, err)
		}
		cfg.applyDefaults(dir)
		if err := writeConfig(writePath, cfg); err != nil {
			return nil, err
		}
		nagf("config: migrated %s -> %s (JSON kept; settings saves go to YAML)", jsonPath, writePath)
		if userJSON && filepath.Clean(userPath) == jsonPath {
			nagf("config: pass --config %s to silence this message", writePath)
		}
		return &Loaded{Path: writePath, WritePath: writePath, Config: *cfg}, nil
	}

	return nil, os.ErrNotExist
}

func createDefaultInDir(dir string) (*Loaded, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("config: ensure config directory %s: %w", dir, err)
	}
	yamlPath, _, _ := configPathsInDir(dir)
	defaultCfg := defaultConfig(dir)
	if err := writeConfig(yamlPath, defaultCfg); err != nil {
		return nil, err
	}
	defaultCfg.applyDefaults(dir)
	return &Loaded{Path: yamlPath, WritePath: yamlPath, Created: true, Config: *defaultCfg}, nil
}

func tryLoadFromResolvedPath(configPath string) (*Loaded, error) {
	configPath = filepath.Clean(strings.TrimSpace(configPath))
	dir := filepath.Dir(configPath)
	loaded, err := resolveLoadedConfig(dir, configPath)
	if err == nil {
		return loaded, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// Explicit single file that is not part of standard trio (e.g. custom name)
	if fileExists(configPath) {
		cfg, rErr := readConfig(configPath)
		if rErr != nil {
			return nil, fmt.Errorf("config: failed to read %s: %w", configPath, rErr)
		}
		cfg.applyDefaults(dir)
		wp := configPath
		ext := strings.ToLower(filepath.Ext(configPath))
		if ext == ".json" {
			yamlPath, _, _ := configPathsInDir(dir)
			if fileExists(yamlPath) {
				wp = yamlPath
				nagf("config: using %s instead of %s", yamlPath, configPath)
			}
		}
		return &Loaded{Path: configPath, WritePath: wp, Config: *cfg}, nil
	}
	return nil, os.ErrNotExist
}

func candidateSearchPaths() ([]string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("config: determine executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	var dirs []string
	dirs = appendIfMissingDir(dirs, execDir)
	if userPath, err := userConfigDir(); err == nil && userPath != "" {
		dirs = appendIfMissingDir(dirs, userPath)
	}
	dirs = appendIfMissingDir(dirs, systemConfigDir)
	dirs = appendIfMissingDir(dirs, filepath.Dir(systemConfigDir)) // /etc for legacy /etc/dnsplane.json
	return dirs, nil
}

func appendIfMissingDir(list []string, dir string) []string {
	dir = filepath.Clean(dir)
	for _, existing := range list {
		if existing == dir {
			return list
		}
	}
	return append(list, dir)
}

func userConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dnsplane"), nil
}

// legacy paths beside /etc/dnsplane.json
func legacyEtcConfig() string {
	return filepath.Join("/etc", FileNameJSON)
}