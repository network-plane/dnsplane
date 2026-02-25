// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package dnsservers

import (
	"testing"
)

func TestServerMatchesQuery(t *testing.T) {
	tests := []struct {
		name      string
		server    DNSServer
		query     string
		wantMatch bool
	}{
		{
			name:      "no whitelist",
			server:    DNSServer{Address: "1.1.1.1", Port: "53"},
			query:     "example.com",
			wantMatch: false,
		},
		{
			name:      "empty whitelist",
			server:    DNSServer{Address: "1.1.1.1", Port: "53", DomainWhitelist: []string{}},
			query:     "example.com",
			wantMatch: false,
		},
		{
			name:      "exact match",
			server:    DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:     "internal.vodafoneinnovus.com",
			wantMatch: true,
		},
		{
			name:      "exact match with trailing dot",
			server:    DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:     "internal.vodafoneinnovus.com.",
			wantMatch: true,
		},
		{
			name:      "subdomain match",
			server:    DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:     "api.internal.vodafoneinnovus.com",
			wantMatch: true,
		},
		{
			name:      "no match",
			server:    DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:     "other.example.com",
			wantMatch: false,
		},
		{
			name:      "multiple whitelist second matches",
			server:    DNSServer{DomainWhitelist: []string{"a.com", "internal.vodafoneinnovus.com"}},
			query:     "api.internal.vodafoneinnovus.com",
			wantMatch: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServerMatchesQuery(tt.server, tt.query)
			if got != tt.wantMatch {
				t.Errorf("ServerMatchesQuery() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestGetServersForQuery(t *testing.T) {
	global := DNSServer{Address: "8.8.8.8", Port: "53", Active: true}
	whitelisted := DNSServer{
		Address:         "192.168.5.5",
		Port:            "53",
		Active:          true,
		DomainWhitelist: []string{"internal.vodafoneinnovus.com"},
	}
	servers := []DNSServer{global, whitelisted}

	// Query for whitelisted domain: only whitelisted server
	got := GetServersForQuery(servers, "api.internal.vodafoneinnovus.com", true)
	if len(got) != 1 || got[0] != "192.168.5.5:53" {
		t.Errorf("GetServersForQuery(whitelisted domain) = %v, want [192.168.5.5:53]", got)
	}

	// Query for other domain: only global server
	got = GetServersForQuery(servers, "example.com", true)
	if len(got) != 1 || got[0] != "8.8.8.8:53" {
		t.Errorf("GetServersForQuery(global domain) = %v, want [8.8.8.8:53]", got)
	}

	// Inactive whitelisted server not returned when activeOnly true
	serversInactive := []DNSServer{global, {Address: "192.168.5.5", Port: "53", Active: false, DomainWhitelist: []string{"internal.vodafoneinnovus.com"}}}
	got = GetServersForQuery(serversInactive, "api.internal.vodafoneinnovus.com", true)
	if len(got) != 0 {
		t.Errorf("GetServersForQuery(whitelisted but inactive, activeOnly=true) = %v, want []", got)
	}

	// Empty server list returns nil
	got = GetServersForQuery(nil, "example.com", true)
	if got != nil {
		t.Errorf("GetServersForQuery(nil, ...) = %v, want nil", got)
	}
	got = GetServersForQuery([]DNSServer{}, "example.com", true)
	if got != nil {
		t.Errorf("GetServersForQuery(empty, ...) = %v, want nil", got)
	}

	// Multiple whitelist servers matching same query: all returned
	whitelist2 := DNSServer{Address: "192.168.5.6", Port: "53", Active: true, DomainWhitelist: []string{"internal.vodafoneinnovus.com"}}
	serversMulti := []DNSServer{global, whitelisted, whitelist2}
	got = GetServersForQuery(serversMulti, "api.internal.vodafoneinnovus.com", true)
	if len(got) != 2 {
		t.Fatalf("GetServersForQuery(two whitelist match) len = %d, want 2", len(got))
	}
	// Both whitelist servers should be present (order not specified)
	seen := make(map[string]bool)
	for _, addr := range got {
		seen[addr] = true
	}
	if !seen["192.168.5.5:53"] || !seen["192.168.5.6:53"] {
		t.Errorf("GetServersForQuery(two whitelist match) = %v", got)
	}

	// activeOnly false: inactive matching server is included
	got = GetServersForQuery(serversInactive, "api.internal.vodafoneinnovus.com", false)
	if len(got) != 1 || got[0] != "192.168.5.5:53" {
		t.Errorf("GetServersForQuery(whitelisted inactive, activeOnly=false) = %v, want [192.168.5.5:53]", got)
	}
}

// FuzzServerMatchesQuery exercises whitelist matching with arbitrary query names.
func FuzzServerMatchesQuery(f *testing.F) {
	server := DNSServer{
		Address:         "192.168.1.1",
		Port:            "53",
		DomainWhitelist: []string{"internal.example.com", "corp.net"},
	}
	f.Add("api.internal.example.com")
	f.Add("internal.example.com.")
	f.Add("")
	f.Fuzz(func(t *testing.T, queryName string) {
		_ = ServerMatchesQuery(server, queryName)
	})
}

// FuzzGetServersForQuery exercises server selection with fuzzed query name.
func FuzzGetServersForQuery(f *testing.F) {
	servers := []DNSServer{
		{Address: "8.8.8.8", Port: "53", Active: true},
		{Address: "192.168.5.5", Port: "53", Active: true, DomainWhitelist: []string{"internal.example.com"}},
	}
	f.Add("example.com")
	f.Add("api.internal.example.com")
	f.Fuzz(func(t *testing.T, queryName string) {
		_ = GetServersForQuery(servers, queryName, true)
		_ = GetServersForQuery(servers, queryName, false)
	})
}
