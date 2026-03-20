// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"testing"
	"time"
)

func TestRecordDashboardResolution_Series(t *testing.T) {
	// Reset-like: new minute keys only; test idempotency of series length
	RecordDashboardResolution(DashboardResolution{
		At:         time.Now().UTC(),
		Qname:      "a.example.",
		Qtype:      "A",
		Outcome:    "cache",
		Record:     "test",
		DurationMs: 1.5,
	})
	s := GetDashboardSeries()
	if len(s) != dashboardSeriesSlots {
		t.Fatalf("GetDashboardSeries len = %d, want %d", len(s), dashboardSeriesSlots)
	}
	log := GetDashboardLogNewestFirst(5)
	if len(log) < 1 {
		t.Fatal("expected at least one log entry")
	}
	if log[0].Qname != "a.example." {
		t.Fatalf("log qname = %q", log[0].Qname)
	}
}
