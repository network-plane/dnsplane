// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"net"
	"strings"
)

// GuessOutboundIPv4 returns the first non-loopback IPv4 address, or empty string.
func GuessOutboundIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

// ClusterDialAddress returns host:port for peers to use when cluster_advertise_addr is empty.
func ClusterDialAddress(listenAddr, advertiseAddr string) string {
	advertiseAddr = strings.TrimSpace(advertiseAddr)
	if advertiseAddr != "" {
		return advertiseAddr
	}
	host := GuessOutboundIPv4()
	if host == "" {
		return ""
	}
	port := "7946"
	if listenAddr != "" {
		if _, p, err := net.SplitHostPort(listenAddr); err == nil && p != "" {
			port = p
		} else if strings.HasPrefix(listenAddr, ":") {
			port = strings.TrimPrefix(listenAddr, ":")
		}
	}
	return net.JoinHostPort(host, port)
}
