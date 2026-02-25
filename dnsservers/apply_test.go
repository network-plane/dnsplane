// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsservers

import (
	"testing"
)

// TestApplyArgsToDNSServer_Whitelist covers plan item 4.4: tests for add/update with and without whitelist.
func TestApplyArgsToDNSServer_Whitelist(t *testing.T) {
	t.Run("add with whitelist", func(t *testing.T) {
		var s DNSServer
		err := applyArgsToDNSServer(&s, []string{"192.168.5.5", "53", "active:true", "whitelist:internal.vodafoneinnovus.com,example.org"})
		if err != nil {
			t.Fatalf("applyArgsToDNSServer: %v", err)
		}
		if s.Address != "192.168.5.5" || s.Port != "53" {
			t.Errorf("address/port: got %s %s", s.Address, s.Port)
		}
		if !s.Active {
			t.Error("expected active true")
		}
		if len(s.DomainWhitelist) != 2 {
			t.Fatalf("DomainWhitelist len = %d, want 2", len(s.DomainWhitelist))
		}
		if s.DomainWhitelist[0] != "internal.vodafoneinnovus.com" || s.DomainWhitelist[1] != "example.org" {
			t.Errorf("DomainWhitelist = %v", s.DomainWhitelist)
		}
	})

	t.Run("add without whitelist", func(t *testing.T) {
		var s DNSServer
		err := applyArgsToDNSServer(&s, []string{"8.8.8.8", "53", "active:true"})
		if err != nil {
			t.Fatalf("applyArgsToDNSServer: %v", err)
		}
		if s.Address != "8.8.8.8" || s.Port != "53" {
			t.Errorf("address/port: got %s %s", s.Address, s.Port)
		}
		if s.DomainWhitelist != nil {
			t.Errorf("DomainWhitelist should be nil when not set, got %v", s.DomainWhitelist)
		}
	})

	t.Run("update sets whitelist", func(t *testing.T) {
		s := DNSServer{Address: "192.168.1.1", Port: "53", Active: true}
		err := applyArgsToDNSServer(&s, []string{"192.168.1.1", "whitelist:corp.net"})
		if err != nil {
			t.Fatalf("applyArgsToDNSServer: %v", err)
		}
		if len(s.DomainWhitelist) != 1 || s.DomainWhitelist[0] != "corp.net" {
			t.Errorf("DomainWhitelist = %v", s.DomainWhitelist)
		}
	})

	t.Run("update clears whitelist with empty value", func(t *testing.T) {
		s := DNSServer{Address: "192.168.1.1", Port: "53", DomainWhitelist: []string{"old.com"}}
		err := applyArgsToDNSServer(&s, []string{"192.168.1.1", "whitelist:"})
		if err != nil {
			t.Fatalf("applyArgsToDNSServer: %v", err)
		}
		if s.DomainWhitelist != nil {
			t.Errorf("DomainWhitelist should be nil when whitelist: empty, got %v", s.DomainWhitelist)
		}
	})

	t.Run("whitelist with spaces trimmed", func(t *testing.T) {
		var s DNSServer
		err := applyArgsToDNSServer(&s, []string{"10.0.0.1", "53", "whitelist: a.com , b.com "})
		if err != nil {
			t.Fatalf("applyArgsToDNSServer: %v", err)
		}
		if len(s.DomainWhitelist) != 2 {
			t.Fatalf("DomainWhitelist len = %d", len(s.DomainWhitelist))
		}
		if s.DomainWhitelist[0] != "a.com" || s.DomainWhitelist[1] != "b.com" {
			t.Errorf("DomainWhitelist = %v", s.DomainWhitelist)
		}
	})

	t.Run("rejects invalid param", func(t *testing.T) {
		var s DNSServer
		err := applyArgsToDNSServer(&s, []string{"192.168.5.5", "unknown:value"})
		if err == nil {
			t.Fatal("expected error for unknown parameter")
		}
	})
}
