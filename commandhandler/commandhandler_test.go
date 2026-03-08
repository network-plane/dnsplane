// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package commandhandler

import (
	"testing"

	"github.com/miekg/dns"
)

func TestLookupRecordTypeToken(t *testing.T) {
	tests := []struct {
		token string
		want  uint16
		ok    bool
	}{
		{"A", dns.TypeA, true},
		{"a", dns.TypeA, true},
		{"AAAA", dns.TypeAAAA, true},
		{"PTR", dns.TypePTR, true},
		{"MX", dns.TypeMX, true},
		{"TXT", dns.TypeTXT, true},
		{"", 0, false},
		{"  ", 0, false},
		{"INVALID", 0, false},
		{"AXFR", dns.TypeAXFR, true},
	}
	for _, tt := range tests {
		got, ok := lookupRecordTypeToken(tt.token)
		if got != tt.want || ok != tt.ok {
			t.Errorf("lookupRecordTypeToken(%q) = (%d, %v), want (%d, %v)", tt.token, got, ok, tt.want, tt.ok)
		}
	}
}

func TestResolveRecordType(t *testing.T) {
	tests := []struct {
		typeToken string
		target    string
		want      uint16
		wantErr   bool
	}{
		{"A", "example.com", dns.TypeA, false},
		{"AAAA", "example.com", dns.TypeAAAA, false},
		{"PTR", "1.2.3.4", dns.TypePTR, false},
		{"", "example.com", dns.TypeA, false},
		{"", "1.2.3.4", dns.TypePTR, false},
		{"", "::1", dns.TypePTR, false},
		{"INVALID", "x.com", 0, true},
	}
	for _, tt := range tests {
		got, err := resolveRecordType(tt.typeToken, tt.target)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolveRecordType(%q, %q) err = %v, wantErr %v", tt.typeToken, tt.target, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("resolveRecordType(%q, %q) = %d, want %d", tt.typeToken, tt.target, got, tt.want)
		}
	}
}

func TestParseDigArguments(t *testing.T) {
	t.Run("missing args", func(t *testing.T) {
		_, _, _, _, err := parseDigArguments(nil)
		if err == nil {
			t.Fatal("parseDigArguments(nil) expected error")
		}
		_, _, _, _, err = parseDigArguments([]string{})
		if err == nil {
			t.Fatal("parseDigArguments([]) expected error")
		}
	})

	t.Run("target only", func(t *testing.T) {
		typeToken, target, host, port, err := parseDigArguments([]string{"example.com"})
		if err != nil {
			t.Fatalf("parseDigArguments: %v", err)
		}
		if typeToken != "" || target != "example.com" || host != "" || port != "" {
			t.Errorf("got typeToken=%q target=%q host=%q port=%q", typeToken, target, host, port)
		}
	})

	t.Run("type and target", func(t *testing.T) {
		typeToken, target, host, port, err := parseDigArguments([]string{"A", "example.com"})
		if err != nil {
			t.Fatalf("parseDigArguments: %v", err)
		}
		if typeToken != "A" || target != "example.com" || host != "" || port != "" {
			t.Errorf("got typeToken=%q target=%q host=%q port=%q", typeToken, target, host, port)
		}
	})

	t.Run("target then type", func(t *testing.T) {
		typeToken, target, host, port, err := parseDigArguments([]string{"example.com", "AAAA"})
		if err != nil {
			t.Fatalf("parseDigArguments: %v", err)
		}
		if typeToken != "AAAA" || target != "example.com" {
			t.Errorf("got typeToken=%q target=%q", typeToken, target)
		}
		if host != "" || port != "" {
			t.Errorf("unexpected host=%q port=%q", host, port)
		}
	})

	t.Run("with server @host:port", func(t *testing.T) {
		typeToken, target, host, port, err := parseDigArguments([]string{"example.com", "@", "192.168.1.1:53"})
		if err != nil {
			t.Fatalf("parseDigArguments: %v", err)
		}
		if host != "192.168.1.1" || port != "53" {
			t.Errorf("got host=%q port=%q, want 192.168.1.1 53", host, port)
		}
		if target != "example.com" {
			t.Errorf("target = %q, want example.com", target)
		}
		_ = typeToken
	})

	t.Run("with server @host", func(t *testing.T) {
		_, target, host, port, err := parseDigArguments([]string{"A", "example.com", "@8.8.8.8"})
		if err != nil {
			t.Fatalf("parseDigArguments: %v", err)
		}
		if host != "8.8.8.8" || target != "example.com" {
			t.Errorf("host=%q target=%q", host, target)
		}
		if port != "" {
			t.Errorf("port = %q, want empty", port)
		}
	})

}
