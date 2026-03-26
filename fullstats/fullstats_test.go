// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package fullstats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_Disabled(t *testing.T) {
	tracker, err := New("", false)
	if err != nil {
		t.Fatalf("New(disabled): %v", err)
	}
	if tracker != nil {
		t.Error("New(disabled) should return nil tracker")
	}
}

func TestNew_Enabled(t *testing.T) {
	dir := t.TempDir()
	tracker, err := New(dir, true)
	if err != nil {
		t.Fatalf("New(enabled): %v", err)
	}
	if tracker == nil {
		t.Fatal("New(enabled) returned nil tracker")
	}
	defer func() { _ = tracker.Close() }()

	dbPath := filepath.Join(dir, dbFileName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file not created: %v", err)
	}
}

func TestTracker_Clear(t *testing.T) {
	dir := t.TempDir()
	tracker, err := New(dir, true)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = tracker.Close() }()

	// Use sync path: RecordRequest is async and can race with GetAllRequests on slow
	// schedulers (notably Windows CI) even with long polls.
	if err := tracker.recordRequestSync("example.com:A", "192.0.2.1", "A", "local"); err != nil {
		t.Fatalf("recordRequestSync: %v", err)
	}
	if err := tracker.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	all, err := tracker.GetAllRequests()
	if err != nil {
		t.Fatalf("GetAllRequests after clear: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("after Clear: len(requests) = %d, want 0", len(all))
	}
	req, err := tracker.GetAllRequesters()
	if err != nil {
		t.Fatalf("GetAllRequesters after clear: %v", err)
	}
	if len(req) != 0 {
		t.Fatalf("after Clear: len(requesters) = %d, want 0", len(req))
	}
	sess, _ := tracker.GetSessionRequests()
	if len(sess) != 0 {
		t.Fatalf("after Clear: len(session requests) = %d, want 0", len(sess))
	}
}
