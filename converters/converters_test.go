// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package converters

import (
	"testing"
)

func TestConvertIPToReverseDNS(t *testing.T) {
	tests := []struct {
		ip   string
		want string
	}{
		{"192.168.1.1", "1.1.168.192.in-addr.arpa"},
		{"127.0.0.1", "1.0.0.127.in-addr.arpa"},
		{"10.0.0.1", "1.0.0.10.in-addr.arpa"},
	}
	for _, tt := range tests {
		got := ConvertIPToReverseDNS(tt.ip)
		if got != tt.want {
			t.Errorf("ConvertIPToReverseDNS(%q) = %q, want %q", tt.ip, got, tt.want)
		}
	}
	// Invalid inputs return a fixed message
	invalid := ConvertIPToReverseDNS("")
	if invalid != "Invalid IP address" {
		t.Errorf("ConvertIPToReverseDNS(\"\") = %q, want \"Invalid IP address\"", invalid)
	}
	invalid = ConvertIPToReverseDNS("1.2.3")
	if invalid != "Invalid IP address" {
		t.Errorf("ConvertIPToReverseDNS(\"1.2.3\") = %q, want \"Invalid IP address\"", invalid)
	}
}

func TestConvertReverseDNSToIP(t *testing.T) {
	tests := []struct {
		reverseDNS string
		want       string
	}{
		{"1.1.168.192.in-addr.arpa", "192.168.1.1"},
		{"1.0.0.127.in-addr.arpa", "127.0.0.1"},
		{"2.0.0.10.in-addr.arpa", "10.0.0.2"},
	}
	for _, tt := range tests {
		got := ConvertReverseDNSToIP(tt.reverseDNS)
		if got != tt.want {
			t.Errorf("ConvertReverseDNSToIP(%q) = %q, want %q", tt.reverseDNS, got, tt.want)
		}
	}
	// Invalid inputs
	invalid := ConvertReverseDNSToIP("")
	if invalid != "Invalid input" {
		t.Errorf("ConvertReverseDNSToIP(\"\") = %q, want \"Invalid input\"", invalid)
	}
	invalid = ConvertReverseDNSToIP("short")
	if invalid != "Invalid input" {
		t.Errorf("ConvertReverseDNSToIP(\"short\") = %q, want \"Invalid input\"", invalid)
	}
}

// FuzzConvertIPToReverseDNS exercises reverse-DNS conversion with arbitrary strings.
func FuzzConvertIPToReverseDNS(f *testing.F) {
	f.Add("192.168.1.1")
	f.Add("127.0.0.1")
	f.Add("")
	f.Fuzz(func(t *testing.T, ip string) {
		_ = ConvertIPToReverseDNS(ip)
	})
}

// FuzzConvertReverseDNSToIP exercises reverse-DNS-to-IP parsing with arbitrary strings.
func FuzzConvertReverseDNSToIP(f *testing.F) {
	f.Add("1.1.168.192.in-addr.arpa")
	f.Add("1.0.0.127.in-addr.arpa")
	f.Add("")
	f.Fuzz(func(t *testing.T, reverseDNS string) {
		_ = ConvertReverseDNSToIP(reverseDNS)
	})
}
