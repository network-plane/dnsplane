// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package dnsservers

import (
	"net"
	"strings"
)

// UpstreamEndpoint describes how to reach an upstream resolver (UDP/TCP/DoT/DoH).
type UpstreamEndpoint struct {
	Addr      string `json:"addr"`      // host:port for udp/tcp/dot; full https URL for doh
	Transport string `json:"transport"` // udp, tcp, dot, doh
}

// HealthKey is the stable key for upstream health tracking and filtering.
func (e UpstreamEndpoint) HealthKey() string {
	return strings.TrimSpace(e.Addr)
}

// String returns a short human-readable form for logs and tables.
func (e UpstreamEndpoint) String() string {
	a := strings.TrimSpace(e.Addr)
	t := strings.ToLower(strings.TrimSpace(e.Transport))
	if t == "" {
		t = "udp"
	}
	if t == "doh" {
		return a
	}
	return a + " [" + t + "]"
}

// ServerToEndpoint maps a DNSServer row to an upstream endpoint.
func ServerToEndpoint(s DNSServer) UpstreamEndpoint {
	t := strings.ToLower(strings.TrimSpace(s.Transport))
	if t == "" {
		t = "udp"
	}
	switch t {
	case "doh":
		url := strings.TrimSpace(s.DoHURL)
		if url == "" {
			url = strings.TrimSpace(s.Address)
		}
		return UpstreamEndpoint{Addr: url, Transport: "doh"}
	case "dot":
		port := strings.TrimSpace(s.Port)
		if port == "" {
			port = "853"
		}
		host := strings.TrimSpace(s.Address)
		addr := net.JoinHostPort(host, port)
		return UpstreamEndpoint{Addr: addr, Transport: "dot"}
	default:
		port := strings.TrimSpace(s.Port)
		if port == "" {
			port = "53"
		}
		host := strings.TrimSpace(s.Address)
		addr := net.JoinHostPort(host, port)
		if t != "tcp" && t != "udp" {
			t = "udp"
		}
		return UpstreamEndpoint{Addr: addr, Transport: t}
	}
}

// FallbackEndpoint builds an endpoint from config-style fallback fields.
func FallbackEndpoint(fallbackIP, fallbackPort, transport string) UpstreamEndpoint {
	t := strings.ToLower(strings.TrimSpace(transport))
	if t == "" {
		t = "udp"
	}
	ip := strings.TrimSpace(fallbackIP)
	port := strings.TrimSpace(fallbackPort)
	if port == "" {
		port = "53"
	}
	addr := net.JoinHostPort(ip, port)
	switch t {
	case "doh":
		// DoH requires a URL; callers should set transport only when URL is in fallback (not supported here).
		return UpstreamEndpoint{Addr: addr, Transport: "udp"}
	case "dot":
		if port == "53" {
			addr = net.JoinHostPort(ip, "853")
		}
		return UpstreamEndpoint{Addr: addr, Transport: "dot"}
	default:
		if t != "tcp" {
			t = "udp"
		}
		return UpstreamEndpoint{Addr: addr, Transport: t}
	}
}
