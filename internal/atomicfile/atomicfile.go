// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

// Package atomicfile writes file contents with temp+fsync+rename so the previous file survives a crash mid-write.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with data using a same-directory temp file, fsync, and atomic rename.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("atomicfile: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return fmt.Errorf("atomicfile: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("atomicfile: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("atomicfile: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("atomicfile: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomicfile: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomicfile: rename %s -> %s: %w", tmpName, path, err)
	}
	cleanup = false
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("atomicfile: sync dir %s: %w", dir, err)
	}
	return nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir) // #nosec G304 -- directory from config path parent
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}