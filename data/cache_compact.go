// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"time"

	"dnsplane/dnsrecordcache"
)

// CompactExpiredCacheRecords removes cache rows whose Expiry is before now (or Expiry is zero).
// It rebuilds the cache index and queues a persist to dnscache.json. Returns how many rows were removed.
func (d *DNSResolverData) CompactExpiredCacheRecords(now time.Time) int {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.CacheRecords) == 0 {
		return 0
	}
	kept := make([]dnsrecordcache.CacheRecord, 0, len(d.CacheRecords))
	removed := 0
	for _, cr := range d.CacheRecords {
		if cr.Expiry.IsZero() || !now.Before(cr.Expiry) {
			removed++
			continue
		}
		kept = append(kept, cr)
	}
	if removed == 0 {
		return 0
	}
	d.CacheRecords = kept
	d.cacheRecordIdx = buildCacheRecordIndex(kept)
	if d.persistCh != nil {
		select {
		case d.persistCh <- struct{}{}:
		default:
		}
	}
	return removed
}

// CacheRecordCount returns the number of rows in the resolver cache (under RLock).
func (d *DNSResolverData) CacheRecordCount() int {
	if d == nil {
		return 0
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.CacheRecords)
}

// SetNextCacheCompactAt sets the expected time of the next scheduled compaction (zero if disabled / unknown).
func (d *DNSResolverData) SetNextCacheCompactAt(t time.Time) {
	if d == nil {
		return
	}
	d.cacheCompactScheduleMu.Lock()
	d.nextCacheCompactAt = t
	d.cacheCompactScheduleMu.Unlock()
}

// NoteCacheCompactRun records completion of a compaction pass.
func (d *DNSResolverData) NoteCacheCompactRun(removed int) {
	if d == nil {
		return
	}
	d.cacheCompactScheduleMu.Lock()
	d.lastCacheCompactAt = time.Now().UTC()
	d.lastCacheCompactRemoved = removed
	d.cacheCompactScheduleMu.Unlock()
}

// CacheCompactSnapshot returns scheduling metadata for dashboards and APIs.
func (d *DNSResolverData) CacheCompactSnapshot() (next, last time.Time, lastRemoved int) {
	if d == nil {
		return time.Time{}, time.Time{}, 0
	}
	d.cacheCompactScheduleMu.RLock()
	defer d.cacheCompactScheduleMu.RUnlock()
	return d.nextCacheCompactAt, d.lastCacheCompactAt, d.lastCacheCompactRemoved
}
