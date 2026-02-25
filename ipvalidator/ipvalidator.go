// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package ipvalidator

import (
	"net"
	"strings"
)

// IPType represents the type of IP address
type IPType int

const (
	Invalid IPType = iota
	IPv4
	IPv6
)

func (t IPType) String() string {
	switch t {
	case IPv4:
		return "IPv4"
	case IPv6:
		return "IPv6"
	default:
		return "Invalid"
	}
}

// IsValidIP returns true if the string is a valid IP address (either IPv4 or IPv6)
func IsValidIP(ip string) bool {
	return ValidateIP(ip) != Invalid
}

// GetIPVersion returns 4 for IPv4, 6 for IPv6, or 0 for invalid IP addresses
func GetIPVersion(ip string) int {
	ipType := ValidateIP(ip)
	switch ipType {
	case IPv4:
		return 4
	case IPv6:
		return 6
	default:
		return 0
	}
}

// ValidateIP checks if the given string is a valid IP address and returns its type
func ValidateIP(ip string) IPType {
	// Remove any leading/trailing whitespace
	ip = strings.TrimSpace(ip)

	// Check if empty
	if ip == "" {
		return Invalid
	}

	// Try parsing as IP address
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return Invalid
	}

	// Check if it's IPv4
	if strings.Contains(ip, ".") {
		// Additional validation for IPv4
		parts := strings.Split(ip, ".")
		if len(parts) != 4 {
			return Invalid
		}

		// Validate each octet
		for _, part := range parts {
			if len(part) == 0 || (len(part) > 1 && part[0] == '0') {
				return Invalid
			}
		}

		return IPv4
	}

	// Check if it's IPv6
	if strings.Contains(ip, ":") {
		// Additional validation for IPv6
		if strings.Count(ip, "::") > 1 {
			return Invalid
		}

		// Validate hexadecimal segments
		parts := strings.Split(strings.ReplaceAll(ip, "::", ":"), ":")
		for _, part := range parts {
			if len(part) > 4 {
				return Invalid
			}
		}

		return IPv6
	}

	return Invalid
}
