package data

import (
	"path/filepath"
	"testing"

	"dnsplane/config"
	"dnsplane/dnsservers"
)

// TestLoadSaveDNSServers_WithWhitelist verifies plan item 1.2: LoadDNSServers/SaveDNSServers
// with domain_whitelist roundtrips correctly.
func TestLoadSaveDNSServers_WithWhitelist(t *testing.T) {
	dir := t.TempDir()
	serversPath := filepath.Join(dir, "dnsservers.json")
	loaded := &config.Loaded{
		Path: filepath.Join(dir, "dnsplane.json"),
		Config: config.Config{
			FileLocations: config.FileLocations{
				DNSServerFile: serversPath,
			},
		},
	}
	SetConfig(loaded)
	defer func() {
		configStateMu.Lock()
		configState = nil
		configStateMu.Unlock()
	}()

	servers := []dnsservers.DNSServer{
		{Address: "8.8.8.8", Port: "53", Active: true},
		{Address: "192.168.5.5", Port: "53", Active: true, DomainWhitelist: []string{"internal.vodafoneinnovus.com"}},
	}
	if err := SaveDNSServers(servers); err != nil {
		t.Fatalf("SaveDNSServers: %v", err)
	}
	loaded2, err := LoadDNSServers()
	if err != nil {
		t.Fatalf("LoadDNSServers: %v", err)
	}
	if len(loaded2) != 2 {
		t.Fatalf("LoadDNSServers len = %d, want 2", len(loaded2))
	}
	if loaded2[0].Address != "8.8.8.8" || loaded2[1].Address != "192.168.5.5" {
		t.Errorf("addresses: %+v", loaded2)
	}
	if len(loaded2[1].DomainWhitelist) != 1 || loaded2[1].DomainWhitelist[0] != "internal.vodafoneinnovus.com" {
		t.Errorf("DomainWhitelist = %v", loaded2[1].DomainWhitelist)
	}
}
