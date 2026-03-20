// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnssecsign

import (
	"path/filepath"
	"testing"
)

func TestLoadSignerInvalidZone(t *testing.T) {
	_, err := LoadSigner(".", "a.key", "a.private")
	if err == nil {
		t.Fatal("expected error for invalid zone")
	}
}

func TestLoadSignerMissingFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadSigner("example.com.", filepath.Join(dir, "nope.key"), filepath.Join(dir, "nope.private"))
	if err == nil {
		t.Fatal("expected error for missing key files")
	}
}
