// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsrecordcache

import (
	"testing"

	"github.com/miekg/dns"
)

func TestAdd_List(t *testing.T) {
	var cache []CacheRecord
	if len(List(cache)) != 0 {
		t.Error("List(empty) should return empty")
	}

	a := &dns.A{
		Hdr: dns.RR_Header{Name: "test.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   []byte{1, 2, 3, 4},
	}
	var rr dns.RR = a
	cache = Add(cache, &rr)
	if len(cache) != 1 {
		t.Fatalf("Add len = %d, want 1", len(cache))
	}
	if cache[0].DNSRecord.Name != "test.example.com." || cache[0].DNSRecord.Type != "A" || cache[0].DNSRecord.Value != "1.2.3.4" {
		t.Errorf("cache[0] = %+v", cache[0])
	}

	list := List(cache)
	if len(list) != 1 {
		t.Errorf("List len = %d", len(list))
	}
}

func TestAdd_Dedupe(t *testing.T) {
	var cache []CacheRecord
	a := &dns.A{
		Hdr: dns.RR_Header{Name: "dup.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   []byte{10, 0, 0, 1},
	}
	var rr dns.RR = a
	cache = Add(cache, &rr)
	cache = Add(cache, &rr)
	if len(cache) != 1 {
		t.Errorf("Add same record twice should dedupe: len = %d", len(cache))
	}
}
