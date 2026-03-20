// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package abuse

import (
	"testing"
	"time"
)

func TestSlidingWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSlidingWindow(time.Second, 2, 100)
	s.now = func() time.Time { return now }

	if !s.Allow("1.1.1.1", "a.") {
		t.Fatal("first allow")
	}
	s.RecordResponse("1.1.1.1", "a.")
	if !s.Allow("1.1.1.1", "a.") {
		t.Fatal("second allow")
	}
	s.RecordResponse("1.1.1.1", "a.")
	if s.Allow("1.1.1.1", "a.") {
		t.Fatal("third should block in same window")
	}
	now = now.Add(2 * time.Second)
	if !s.Allow("1.1.1.1", "a.") {
		t.Fatal("allow after window reset")
	}
}

func TestRRLMaxPerBucket(t *testing.T) {
	r := NewRRL(2, time.Second, 0, 100)
	r.now = func() time.Time { return time.Unix(3000, 0) }

	if !r.Allow("1.1.1.1", "x.") {
		t.Fatal("first")
	}
	r.RecordResponse("1.1.1.1", "x.")
	if !r.Allow("1.1.1.1", "x.") {
		t.Fatal("second")
	}
	r.RecordResponse("1.1.1.1", "x.")
	if r.Allow("1.1.1.1", "x.") {
		t.Fatal("third should block")
	}
}
