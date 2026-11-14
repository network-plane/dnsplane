// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	orig := []byte(`{"cache":[]}`)
	if err := Write(path, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatalf("content mismatch: %q", got)
	}
	next := []byte(`{"cache":[{"x":1}]}`)
	if err := Write(path, next, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatalf("after replace: %q", got)
	}
}