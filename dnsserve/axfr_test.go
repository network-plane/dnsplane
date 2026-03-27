// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package dnsserve

import (
	"context"
	"testing"

	"dnsplane/config"
	"dnsplane/dnsrecords"

	"github.com/miekg/dns"
)

func TestTryServeAXFRUDPNotImplemented(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeAXFR)
	req.Opcode = dns.OpcodeQuery
	dep := Dependencies{
		Settings: func() config.Config { return config.Config{} },
	}
	resp, ok := tryServeAXFR(req, ServeMeta{ClientIP: "127.0.0.1", Protocol: ProtoUDP}, dep)
	if !ok || resp.Rcode != dns.RcodeNotImplemented {
		t.Fatalf("got ok=%v rcode=%v", ok, resp.Rcode)
	}
}

func TestServeDNSUpdateNotImplemented(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Opcode = dns.OpcodeUpdate
	dep := Dependencies{Settings: func() config.Config { return config.Config{} }}
	resp := ServeDNS(context.Background(), req, ServeMeta{ClientIP: "1.1.1.1", Protocol: ProtoUDP}, dep)
	if resp.Rcode != dns.RcodeNotImplemented {
		t.Fatalf("want NOTIMP, got rcode=%d", resp.Rcode)
	}
}

func TestAxfrRecordsForZoneOrdering(t *testing.T) {
	recs := []dnsrecords.DNSRecord{
		{Name: "www.example.com", Type: "A", Value: "192.0.2.1", TTL: 60},
		{Name: "example.com", Type: "SOA", Value: "ns1.example.com. hostmaster.example.com. 1 7200 900 1209600 3600", TTL: 3600},
	}
	rrs, err := axfrRecordsForZone(recs, "example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 3 {
		t.Fatalf("want 3 RRs (SOA + A + SOA), got %d", len(rrs))
	}
	if rrs[0].Header().Rrtype != dns.TypeSOA || rrs[len(rrs)-1].Header().Rrtype != dns.TypeSOA {
		t.Fatal("expected SOA first and last")
	}
}
