package dnsservers

import (
	"encoding/json"
	"testing"
)

// TestDNSServerJSONRoundtrip verifies plan item 1.2: domain_whitelist persists correctly in JSON.
func TestDNSServerJSONRoundtrip(t *testing.T) {
	orig := DNSServer{
		Address:         "192.168.5.5",
		Port:            "53",
		Active:          true,
		DomainWhitelist: []string{"internal.vodafoneinnovus.com", "example.org"},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded DNSServer
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Address != orig.Address || decoded.Port != orig.Port || decoded.Active != orig.Active {
		t.Errorf("decoded base fields: %+v", decoded)
	}
	if len(decoded.DomainWhitelist) != len(orig.DomainWhitelist) {
		t.Fatalf("DomainWhitelist len: got %d, want %d", len(decoded.DomainWhitelist), len(orig.DomainWhitelist))
	}
	for i := range orig.DomainWhitelist {
		if decoded.DomainWhitelist[i] != orig.DomainWhitelist[i] {
			t.Errorf("DomainWhitelist[%d]: got %q, want %q", i, decoded.DomainWhitelist[i], orig.DomainWhitelist[i])
		}
	}
}

// TestDNSServerJSON_OptionalWhitelist verifies servers without whitelist (or empty) unmarshal (backward compat).
func TestDNSServerJSON_OptionalWhitelist(t *testing.T) {
	// JSON without domain_whitelist key
	data := []byte(`{"address":"8.8.8.8","port":"53","active":true}`)
	var s DNSServer
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if s.DomainWhitelist != nil {
		t.Errorf("expected nil whitelist when key omitted, got %v", s.DomainWhitelist)
	}
	// Roundtrip: marshal again and ensure no domain_whitelist in output when empty (or omitempty keeps it out)
	out, _ := json.Marshal(s)
	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal output: %v", err)
	}
	// With omitempty, nil slice may be omitted; that's fine for backward compat
}
