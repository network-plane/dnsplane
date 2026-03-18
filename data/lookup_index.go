// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"

	"dnsplane/dnsrecords"
)

func dnsCacheIdxKey(name, recordType string) string {
	return dnsrecords.NormalizeRecordNameKey(name) + "\x00" + dnsrecords.NormalizeRecordType(recordType)
}

func (d *DNSResolverData) rebuildCacheIndexLocked() {
	d.cacheRecordIdx = make(map[string][]int)
	for i := range d.CacheRecords {
		cr := &d.CacheRecords[i]
		k := dnsCacheIdxKey(cr.DNSRecord.Name, cr.DNSRecord.Type)
		d.cacheRecordIdx[k] = append(d.cacheRecordIdx[k], i)
	}
}

func (d *DNSResolverData) rebuildDNSRecordIndexLocked() {
	d.dnsRecordIdx = make(map[string][]int)
	for i := range d.DNSRecords {
		rec := &d.DNSRecords[i]
		k := dnsCacheIdxKey(rec.Name, rec.Type)
		d.dnsRecordIdx[k] = append(d.dnsRecordIdx[k], i)
	}
}

// LookupCacheRR returns the first non-expired cached RR for name+type, or nil.
func (d *DNSResolverData) LookupCacheRR(qname, recordType string) *dns.RR {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.CacheRecords) == 0 {
		return nil
	}
	k := dnsCacheIdxKey(qname, recordType)
	now := time.Now()
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

// LookupLocalRRs returns local RRs for name+type. PTR (and auto-build PTR) uses a full scan.
func (d *DNSResolverData) LookupLocalRRs(qname, recordType string, autoBuildPTRFromA bool) []dns.RR {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rt := strings.ToUpper(strings.TrimSpace(recordType))
	if rt == "PTR" {
		return dnsrecords.FindAllRecords(d.DNSRecords, qname, recordType, autoBuildPTRFromA)
	}
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
