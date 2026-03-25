// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"testing"
	"time"

	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
)

func TestCompactExpiredCacheRecords(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)
	d := &DNSResolverData{
		CacheRecords: []dnsrecordcache.CacheRecord{
			{DNSRecord: dnsrecords.DNSRecord{Name: "a.", Type: "A", Value: "1.1.1.1", TTL: 60}, Expiry: past},
			{DNSRecord: dnsrecords.DNSRecord{Name: "b.", Type: "A", Value: "2.2.2.2", TTL: 60}, Expiry: future},
			{DNSRecord: dnsrecords.DNSRecord{Name: "z.", Type: "A", Value: "9.9.9.9", TTL: 60}}, // zero expiry → removed
		},
	}
	d.rebuildCacheIndexLocked()
	n := d.CompactExpiredCacheRecords(time.Now())
	if n != 2 {
		t.Fatalf("removed %d, want 2", n)
	}
	if len(d.CacheRecords) != 1 {
		t.Fatalf("len %d, want 1", len(d.CacheRecords))
	}
	if d.CacheRecords[0].DNSRecord.Name != "b." {
		t.Fatalf("kept wrong row: %+v", d.CacheRecords[0].DNSRecord)
	}
}
