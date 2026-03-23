// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"

	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
)

// RRSetCachePrefix marks a cache Value that holds multiple RRs (e.g. CNAME chain + A/AAAA) for the query name+type.
const RRSetCachePrefix = "__RRSET_v1__\n"

// BuildRRSetCacheValue encodes answer RRs for synthetic (qname, A|AAAA) cache storage.
func BuildRRSetCacheValue(rrs []dns.RR) string {
	var b strings.Builder
	b.WriteString(RRSetCachePrefix)
	for _, rr := range rrs {
		b.WriteString(rr.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func parseRRSetCacheValue(v string) ([]dns.RR, bool) {
	if !strings.HasPrefix(v, RRSetCachePrefix) {
		return nil, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(v, RRSetCachePrefix))
	if rest == "" {
		return nil, false
	}
	lines := strings.Split(rest, "\n")
	var out []dns.RR
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rr, err := dns.NewRR(line)
		if err != nil {
			return nil, false
		}
		out = append(out, rr)
	}
	return out, len(out) > 0
}

func clipRRSetTTLs(rrs []dns.RR, remainingSec uint32, stale bool) []dns.RR {
	out := make([]dns.RR, len(rrs))
	for i, rr := range rrs {
		cp := dns.Copy(rr)
		hdr := cp.Header()
		if stale {
			hdr.Ttl = 1
		} else if remainingSec < hdr.Ttl {
			hdr.Ttl = remainingSec
		}
		if hdr.Ttl == 0 {
			hdr.Ttl = 1
		}
		out[i] = cp
	}
	return out
}

func dnsCacheIdxKey(name, recordType string) string {
	return dnsrecords.NormalizeRecordNameKey(name) + "\x00" + dnsrecords.NormalizeRecordType(recordType)
}

// buildDNSRecordIndex builds the local-records lookup map (caller must not hold d.mu during build).
func buildDNSRecordIndex(records []dnsrecords.DNSRecord) map[string][]int {
	idx := make(map[string][]int)
	for i := range records {
		rec := &records[i]
		k := dnsCacheIdxKey(rec.Name, rec.Type)
		idx[k] = append(idx[k], i)
	}
	return idx
}

// buildCacheRecordIndex builds the cache lookup map (caller must not hold d.mu during build).
func buildCacheRecordIndex(records []dnsrecordcache.CacheRecord) map[string][]int {
	idx := make(map[string][]int)
	for i := range records {
		cr := &records[i]
		k := dnsCacheIdxKey(cr.DNSRecord.Name, cr.DNSRecord.Type)
		idx[k] = append(idx[k], i)
	}
	return idx
}

func (d *DNSResolverData) rebuildCacheIndexLocked() {
	d.cacheRecordIdx = buildCacheRecordIndex(d.CacheRecords)
}

func (d *DNSResolverData) rebuildDNSRecordIndexLocked() {
	d.dnsRecordIdx = buildDNSRecordIndex(d.DNSRecords)
}

// lookupCacheRRLocked requires d.mu RLock held.
// When stale is true, returns expired entries with TTL=1 (for stale-while-revalidate).
// The second return value is true when the returned entry is stale (expired).
func (d *DNSResolverData) lookupCacheRRLocked(qname, recordType string, now time.Time, stale bool) (*dns.RR, bool) {
	if len(d.CacheRecords) == 0 {
		return nil, false
	}
	k := dnsCacheIdxKey(qname, recordType)
	var bestStale *dnsrecords.DNSRecord
	for _, i := range d.cacheRecordIdx[k] {
		if i < 0 || i >= len(d.CacheRecords) {
			continue
		}
		rec := &d.CacheRecords[i]
		if strings.HasPrefix(rec.DNSRecord.Value, RRSetCachePrefix) {
			continue
		}
		if now.Before(rec.Expiry) {
			ttl := uint32(rec.Expiry.Sub(now).Seconds())
			return dnsRecordToRRForLookup(&rec.DNSRecord, ttl, nil), false
		}
		if stale && bestStale == nil {
			bestStale = &rec.DNSRecord
		}
	}
	for i := range d.CacheRecords {
		rec := &d.CacheRecords[i]
		if dnsrecords.NormalizeRecordNameKey(rec.DNSRecord.Name) != dnsrecords.NormalizeRecordNameKey(qname) {
			continue
		}
		if dnsrecords.NormalizeRecordType(rec.DNSRecord.Type) != dnsrecords.NormalizeRecordType(recordType) {
			continue
		}
		if strings.HasPrefix(rec.DNSRecord.Value, RRSetCachePrefix) {
			continue
		}
		if now.Before(rec.Expiry) {
			ttl := uint32(rec.Expiry.Sub(now).Seconds())
			return dnsRecordToRRForLookup(&rec.DNSRecord, ttl, nil), false
		}
		if stale && bestStale == nil {
			bestStale = &rec.DNSRecord
		}
	}
	if bestStale != nil {
		return dnsRecordToRRForLookup(bestStale, 1, nil), true
	}
	return nil, false
}

// lookupRRSetCacheLocked requires d.mu RLock held. Returns a synthetic multi-RR answer for (qname, A|AAAA).
func (d *DNSResolverData) lookupRRSetCacheLocked(qname, recordType string, now time.Time, stale bool) ([]dns.RR, bool) {
	rt := dnsrecords.NormalizeRecordType(recordType)
	if rt != "A" && rt != "AAAA" {
		return nil, false
	}
	if len(d.CacheRecords) == 0 {
		return nil, false
	}
	k := dnsCacheIdxKey(qname, recordType)
	var bestStale *dnsrecordcache.CacheRecord
	for _, i := range d.cacheRecordIdx[k] {
		if i < 0 || i >= len(d.CacheRecords) {
			continue
		}
		cr := &d.CacheRecords[i]
		if !strings.HasPrefix(cr.DNSRecord.Value, RRSetCachePrefix) {
			continue
		}
		rrs, ok := parseRRSetCacheValue(cr.DNSRecord.Value)
		if !ok {
			continue
		}
		if now.Before(cr.Expiry) {
			ttl := uint32(cr.Expiry.Sub(now).Seconds())
			if ttl == 0 {
				ttl = 1
			}
			return clipRRSetTTLs(rrs, ttl, false), false
		}
		if stale && bestStale == nil {
			bestStale = cr
		}
	}
	for i := range d.CacheRecords {
		cr := &d.CacheRecords[i]
		if dnsrecords.NormalizeRecordNameKey(cr.DNSRecord.Name) != dnsrecords.NormalizeRecordNameKey(qname) {
			continue
		}
		if dnsrecords.NormalizeRecordType(cr.DNSRecord.Type) != rt {
			continue
		}
		if !strings.HasPrefix(cr.DNSRecord.Value, RRSetCachePrefix) {
			continue
		}
		rrs, ok := parseRRSetCacheValue(cr.DNSRecord.Value)
		if !ok {
			continue
		}
		if now.Before(cr.Expiry) {
			ttl := uint32(cr.Expiry.Sub(now).Seconds())
			if ttl == 0 {
				ttl = 1
			}
			return clipRRSetTTLs(rrs, ttl, false), false
		}
		if stale && bestStale == nil {
			bestStale = cr
		}
	}
	if bestStale != nil {
		rrs, ok := parseRRSetCacheValue(bestStale.DNSRecord.Value)
		if !ok || len(rrs) == 0 {
			return nil, false
		}
		return clipRRSetTTLs(rrs, 1, true), true
	}
	return nil, false
}

// LookupCacheRR returns the first non-expired cached RR for name+type, or nil.
func (d *DNSResolverData) LookupCacheRR(qname, recordType string) *dns.RR {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rr, _ := d.lookupCacheRRLocked(qname, recordType, time.Now(), false)
	return rr
}

// lookupLocalNonPTRLocked requires d.mu RLock held; not for PTR qtype.
func (d *DNSResolverData) lookupLocalNonPTRLocked(qname, recordType string) []dns.RR {
	if len(d.DNSRecords) == 0 {
		return nil
	}
	k := dnsCacheIdxKey(qname, recordType)
	idxs := d.dnsRecordIdx[k]
	if len(idxs) == 0 {
		return nil
	}
	var out []dns.RR
	for _, i := range idxs {
		if i < 0 || i >= len(d.DNSRecords) {
			continue
		}
		rec := d.DNSRecords[i]
		rrString := fmt.Sprintf("%s %d IN %s %s", rec.Name, rec.TTL, rec.Type, rec.Value)
		rr, err := dns.NewRR(rrString)
		if err == nil {
			out = append(out, rr)
		}
	}
	return out
}

// TryFastLocalOrCache does local-then-cache under a single RLock. For PTR queries returns handled=false.
// When stale-while-revalidate is enabled, expired cache entries are returned (with isStale=true)
// so the caller can serve them immediately and refresh in the background.
// cacheRRs is set when the answer is a synthetic RRset (e.g. CNAME chain + A/AAAA) keyed by (qname, A|AAAA).
func (d *DNSResolverData) TryFastLocalOrCache(qname, recordType string, qtypePTR bool) (handled bool, local []dns.RR, cache *dns.RR, cacheRRs []dns.RR, isStale bool) {
	if qtypePTR {
		return false, nil, nil, nil, false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.DNSRecords) > 0 {
		local = d.lookupLocalNonPTRLocked(qname, recordType)
		if len(local) > 0 {
			return true, local, nil, nil, false
		}
	}
	if !d.Settings.CacheRecords || len(d.CacheRecords) == 0 {
		return false, nil, nil, nil, false
	}
	allowStale := d.Settings.StaleWhileRevalidate
	now := time.Now()
	cache, isStale = d.lookupCacheRRLocked(qname, recordType, now, allowStale)
	if cache != nil {
		return true, nil, cache, nil, isStale
	}
	if rrset, st := d.lookupRRSetCacheLocked(qname, recordType, now, allowStale); len(rrset) > 0 {
		return true, nil, nil, rrset, st
	}
	return false, nil, nil, nil, false
}

// LookupLocalRRs returns local RRs for name+type. PTR (and auto-build PTR) uses a full scan.
func (d *DNSResolverData) LookupLocalRRs(qname, recordType string, autoBuildPTRFromA bool) []dns.RR {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rt := strings.ToUpper(strings.TrimSpace(recordType))
	if rt == "PTR" {
		return dnsrecords.FindAllRecords(d.DNSRecords, qname, recordType, autoBuildPTRFromA)
	}
	return d.lookupLocalNonPTRLocked(qname, recordType)
}

func dnsRecordToRRForLookup(dr *dnsrecords.DNSRecord, ttl uint32, errLog func(msg string, kv ...any)) *dns.RR {
	s := fmt.Sprintf("%s %d IN %s %s", dr.Name, ttl, dr.Type, dr.Value)
	rr, err := dns.NewRR(s)
	if err != nil {
		if errLog != nil {
			errLog("data: cache RR parse", "error", err)
		}
		return nil
	}
	return &rr
}

// WarmIndexes rebuilds lookup indexes (e.g. after tests manipulate slices without store*).
func (d *DNSResolverData) WarmIndexes() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rebuildDNSRecordIndexLocked()
	d.rebuildCacheIndexLocked()
}

// HasAnyLocalRecords reports whether any local DNS records exist (cheap; for resolver short-circuit).
func (d *DNSResolverData) HasAnyLocalRecords() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.DNSRecords) > 0
}

// HasAnyCachedRecords reports whether the cache has any entries (cheap; skip cache lookup when empty).
func (d *DNSResolverData) HasAnyCachedRecords() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.CacheRecords) > 0
}
