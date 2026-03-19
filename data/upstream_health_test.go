// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"testing"
	"time"

	"dnsplane/config"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
)

func TestTryFastLocalOrCache_CacheHitWithLocalZoneElsewhere(t *testing.T) {
	d := &DNSResolverData{
		DNSRecords: []dnsrecords.DNSRecord{
			{Name: "other.internal.", Type: "A", Value: "10.0.0.1", TTL: 60},
		},
		Settings: config.Config{CacheRecords: true},
		CacheRecords: []dnsrecordcache.CacheRecord{
			{
				DNSRecord: dnsrecords.DNSRecord{Name: "example.com.", Type: "A", Value: "93.184.216.34", TTL: 60},
				Expiry:    time.Now().Add(time.Hour),
			},
		},
	}
	d.mu.Lock()
	d.rebuildDNSRecordIndexLocked()
	d.rebuildCacheIndexLocked()
	d.mu.Unlock()

	ok, loc, crr, _ := d.TryFastLocalOrCache("example.com.", "A", false)
	if !ok || len(loc) != 0 || crr == nil {
		t.Fatalf("want cache hit, got ok=%v loc=%d crr=%v", ok, len(loc), crr != nil)
	}
}

func TestUpstreamHealthTracker_Filter(t *testing.T) {
	tr := NewUpstreamHealthTracker()
	cfg := &config.Config{UpstreamHealthCheckEnabled: true}
	addrs := []string{"8.8.8.8:53", "1.1.1.1:53"}
	if got := tr.Filter(cfg, addrs); len(got) != 2 {
		t.Fatalf("want 2 servers, got %d", len(got))
	}
	for i := 0; i < 3; i++ {
		tr.ProbeFail("8.8.8.8:53", "timeout", 3)
	}
	if got := tr.Filter(cfg, addrs); len(got) != 1 || got[0] != "1.1.1.1:53" {
		t.Fatalf("after fail: %v", got)
	}
	tr.ProbeOK("8.8.8.8:53")
	if got := tr.Filter(cfg, addrs); len(got) != 2 {
		t.Fatalf("after ok: want 2, got %d", len(got))
	}
}

func TestUpstreamHealthTracker_FilterAllUnhealthyFallback(t *testing.T) {
	tr := NewUpstreamHealthTracker()
	cfg := &config.Config{UpstreamHealthCheckEnabled: true}
	addrs := []string{"9.9.9.9:53"}
	tr.ProbeFail("9.9.9.9:53", "down", 1)
	if got := tr.Filter(cfg, addrs); len(got) != 1 {
		t.Fatalf("degenerate fallback: want 1, got %v", got)
	}
}

func TestUpstreamHealthTracker_DisabledPassesThrough(t *testing.T) {
	tr := NewUpstreamHealthTracker()
	cfg := &config.Config{UpstreamHealthCheckEnabled: false}
	tr.ProbeFail("8.8.8.8:53", "x", 1)
	addrs := []string{"8.8.8.8:53"}
	if got := tr.Filter(cfg, addrs); len(got) != 1 {
		t.Fatal("disabled should not filter")
	}
}
