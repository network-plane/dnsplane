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
func (d *DNSResolverData) lookupCacheRRLocked(qname, recordType string, now time.Time) *dns.RR {
	if len(d.CacheRecords) == 0 {
		return nil
	}
	k := dnsCacheIdxKey(qname, recordType)
	for _, i := range d.cacheRecordIdx[k] {
		if i < 0 || i >= len(d.CacheRecords) {
			continue
		}
		rec := &d.CacheRecords[i]
		if now.Before(rec.Expiry) {
			ttl := uint32(rec.Expiry.Sub(now).Seconds())
			return dnsRecordToRRForLookup(&rec.DNSRecord, ttl, nil)
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
		if now.Before(rec.Expiry) {
			ttl := uint32(rec.Expiry.Sub(now).Seconds())
			return dnsRecordToRRForLookup(&rec.DNSRecord, ttl, nil)
		}
	}
	return nil
}

// LookupCacheRR returns the first non-expired cached RR for name+type, or nil.
func (d *DNSResolverData) LookupCacheRR(qname, recordType string) *dns.RR {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lookupCacheRRLocked(qname, recordType, time.Now())
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
func (d *DNSResolverData) TryFastLocalOrCache(qname, recordType string, qtypePTR bool) (handled bool, local []dns.RR, cache *dns.RR) {
	if qtypePTR {
		return false, nil, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.DNSRecords) > 0 {
		local = d.lookupLocalNonPTRLocked(qname, recordType)
		if len(local) > 0 {
			return true, local, nil
		}
	}
	if !d.Settings.CacheRecords || len(d.CacheRecords) == 0 {
		return false, nil, nil
	}
	cache = d.lookupCacheRRLocked(qname, recordType, time.Now())
	if cache != nil {
		return true, nil, cache
	}
	return false, nil, nil
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
