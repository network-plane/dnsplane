// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsservers_test

import (
	"testing"

	"dnsplane/dnsservers"
)

func TestAppendPerServerFallbacks(t *testing.T) {
	primary := dnsservers.DNSServer{
		Address:           "10.0.0.1",
		Port:              "53",
		Active:            true,
		DomainWhitelist:   []string{"int.example"},
		FallbackAddress:   "10.0.0.2",
		FallbackPort:      "53",
		FallbackTransport: "udp",
	}
	servers := []dnsservers.DNSServer{primary}
	prim := dnsservers.GetUpstreamEndpointsForQuery(servers, "x.int.example.", true)
	if len(prim) != 1 {
		t.Fatalf("primaries: %d", len(prim))
	}
	out := dnsservers.AppendPerServerFallbacks(prim, servers)
	if len(out) != 2 {
		t.Fatalf("want 2 endpoints, got %d: %+v", len(out), out)
	}
	// Fallback with same addr as primary is skipped (already in seen).
	sameFB := primary
	sameFB.FallbackAddress = "10.0.0.1"
	sameFB.FallbackPort = "53"
	outSame := dnsservers.AppendPerServerFallbacks(prim, []dnsservers.DNSServer{sameFB})
	if len(outSame) != 1 {
		t.Fatalf("same fallback as primary: want 1 endpoint, got %d", len(outSame))
	}
}
