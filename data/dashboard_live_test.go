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

func TestSetDashboardResolutionLogCap_RingSize(t *testing.T) {
	t.Cleanup(func() { SetDashboardResolutionLogCap(1000) })
	SetDashboardResolutionLogCap(3)
	for i := 0; i < 5; i++ {
		RecordDashboardResolution(DashboardResolution{
			At:         time.Now().UTC(),
			Qname:      "ring.example.",
			Qtype:      "A",
			Outcome:    "ok",
			Record:     "1.2.3.4",
			DurationMs: 1,
		})
	}
	got := GetDashboardLogNewestFirst(100)
	if len(got) != 3 {
		t.Fatalf("GetDashboardLogNewestFirst: len = %d, want 3", len(got))
	}
	if DashboardLogCap() != 3 {
		t.Fatalf("DashboardLogCap = %d, want 3", DashboardLogCap())
	}
}

func TestClearDashboardResolutionLog(t *testing.T) {
	t.Cleanup(func() { SetDashboardResolutionLogCap(1000) })
	SetDashboardResolutionLogCap(1000)
	RecordDashboardResolution(DashboardResolution{
		At: time.Now().UTC(), Qname: "clear.test.", Qtype: "A", Outcome: "cache", Record: "1.2.3.4", DurationMs: 1,
	})
	ClearDashboardResolutionLog()
	if n := len(GetDashboardLogNewestFirst(100)); n != 0 {
		t.Fatalf("GetDashboardLogNewestFirst: len = %d, want 0", n)
	}
}
