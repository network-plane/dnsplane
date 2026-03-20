// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"strings"
	"testing"
)

func TestCumulativeFromBuckets(t *testing.T) {
	counts := [8]uint64{1, 2, 3, 0, 0, 0, 0, 0}
	cum := cumulativeFromBuckets(counts)
	want := [8]uint64{1, 3, 6, 6, 6, 6, 6, 6}
	if cum != want {
		t.Fatalf("cumulativeFromBuckets = %v, want %v", cum, want)
	}
}

func TestWriteResolverPerfPrometheus_Empty(t *testing.T) {
	var b strings.Builder
	WriteResolverPerfPrometheus(&b)
	out := b.String()
	if !strings.Contains(out, "dnsplane_dns_resolve_duration_seconds") {
		t.Fatalf("expected histogram metric in output: %q", out)
	}
	if !strings.Contains(out, "dnsplane_dns_resolve_duration_seconds_count 0") {
		t.Fatalf("expected zero count: %q", out)
	}
}
