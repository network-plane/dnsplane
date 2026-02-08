package dnsservers

import (
	"testing"
)

func TestServerMatchesQuery(t *testing.T) {
	tests := []struct {
		name     string
		server   DNSServer
		query    string
		wantMatch bool
	}{
		{
			name:     "no whitelist",
			server:   DNSServer{Address: "1.1.1.1", Port: "53"},
			query:    "example.com",
			wantMatch: false,
		},
		{
			name:     "empty whitelist",
			server:   DNSServer{Address: "1.1.1.1", Port: "53", DomainWhitelist: []string{}},
			query:    "example.com",
			wantMatch: false,
		},
		{
			name:     "exact match",
			server:   DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:    "internal.vodafoneinnovus.com",
			wantMatch: true,
		},
		{
			name:     "exact match with trailing dot",
			server:   DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:    "internal.vodafoneinnovus.com.",
			wantMatch: true,
		},
		{
			name:     "subdomain match",
			server:   DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:    "api.internal.vodafoneinnovus.com",
			wantMatch: true,
		},
		{
			name:     "no match",
			server:   DNSServer{DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
			query:    "other.example.com",
			wantMatch: false,
		},
		{
			name:     "multiple whitelist second matches",
			server:   DNSServer{DomainWhitelist: []string{"a.com", "internal.vodafoneinnovus.com"}},
			query:    "api.internal.vodafoneinnovus.com",
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

	// Inactive whitelisted server not returned
	serversInactive := []DNSServer{global, {Address: "192.168.5.5", Port: "53", Active: false, DomainWhitelist: []string{"internal.vodafoneinnovus.com"}}}
	got = GetServersForQuery(serversInactive, "api.internal.vodafoneinnovus.com", true)
	if len(got) != 0 {
		t.Errorf("GetServersForQuery(whitelisted but inactive) = %v, want []", got)
	}
}
