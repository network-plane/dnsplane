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

func TestGetDashboardPerSecRates(t *testing.T) {
	t.Cleanup(func() { SetDashboardResolutionLogCap(1000) })
	SetDashboardResolutionLogCap(1000)
	// Rates use completed UTC seconds [now-1 … now-5], not the current second.
	prev := time.Unix(time.Now().UTC().Unix()-1, 0).UTC()
	RecordDashboardResolution(DashboardResolution{
		At: prev, Qname: "a.", Qtype: "A", Outcome: "cache", Record: "1.1.1.1", DurationMs: 1,
	})
	RecordDashboardResolution(DashboardResolution{
		At: prev, Qname: "b.", Qtype: "A", Outcome: "upstream", Record: "2.2.2.2", DurationMs: 2,
	})
	r := GetDashboardPerSecRates(5)
	if r.WindowSeconds != 5 {
		t.Fatalf("WindowSeconds = %d", r.WindowSeconds)
	}
	if r.Resolutions < 0.35 || r.Resolutions > 0.45 {
		t.Fatalf("Resolutions per sec = %v (want ~0.4 for 2 events / 5s)", r.Resolutions)
	}
	if r.Cache < 0.15 || r.Cache > 0.25 {
		t.Fatalf("Cache per sec = %v", r.Cache)
	}
	if r.Upstream < 0.15 || r.Upstream > 0.25 {
		t.Fatalf("Upstream per sec = %v", r.Upstream)
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
