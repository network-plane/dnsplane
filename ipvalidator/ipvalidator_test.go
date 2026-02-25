// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package ipvalidator

import (
	"testing"
)

func TestIsValidIP(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "192.168.1.1"} {
		if !IsValidIP(ip) {
			t.Errorf("IsValidIP(%q) = false, want true", ip)
		}
	}
	for _, ip := range []string{"", "not-an-ip", "256.1.1.1"} {
		if IsValidIP(ip) {
			t.Errorf("IsValidIP(%q) = true, want false", ip)
		}
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want IPType
	}{
		{"127.0.0.1", IPv4},
		{"192.168.1.1", IPv4},
		{" 10.0.0.1 ", IPv4},
		{"::1", IPv6},
		{"2001:db8::1", IPv6},
		{"fe80::1", IPv6},
		{"", Invalid},
		{"not-an-ip", Invalid},
		{"256.1.1.1", Invalid},
		{"1.2.3.4.5", Invalid},
		{"01.2.3.4", Invalid},
		{"1.2.3", Invalid},
	}
	for _, tt := range tests {
		got := ValidateIP(tt.ip)
		if got != tt.want {
			t.Errorf("ValidateIP(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestGetIPVersion(t *testing.T) {
	tests := []struct {
		ip   string
		want int
	}{
		{"127.0.0.1", 4},
		{"::1", 6},
		{"192.168.0.1", 4},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := GetIPVersion(tt.ip)
		if got != tt.want {
			t.Errorf("GetIPVersion(%q) = %d, want %d", tt.ip, got, tt.want)
		}
	}
}

func TestIPTypeString(t *testing.T) {
	if IPv4.String() != "IPv4" {
		t.Errorf("IPv4.String() = %q, want IPv4", IPv4.String())
	}
	if IPv6.String() != "IPv6" {
		t.Errorf("IPv6.String() = %q, want IPv6", IPv6.String())
	}
	if Invalid.String() != "Invalid" {
		t.Errorf("Invalid.String() = %q, want Invalid", Invalid.String())
	}
}

// FuzzValidateIP exercises IP validation with arbitrary strings to find panics or bugs.
func FuzzValidateIP(f *testing.F) {
	f.Add("127.0.0.1")
	f.Add("::1")
	f.Add("192.168.0.1")
	f.Add("")
	f.Fuzz(func(t *testing.T, ip string) {
		_ = ValidateIP(ip)
		_ = IsValidIP(ip)
		_ = GetIPVersion(ip)
	})
}
