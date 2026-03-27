// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"net"
	"sort"
	"strconv"
	"strings"
)

// SRVLookup matches net.LookupSRV for dependency injection in tests.
type SRVLookup func(service, proto, name string) (cname string, addrs []*net.SRV, err error)

// ParseSRVQuery parses a name like _dnsplane._tcp.example.com. into LookupSRV arguments.
func ParseSRVQuery(fqdn string) (service, proto, zone string, ok bool) {
	fqdn = strings.TrimSpace(fqdn)
	fqdn = strings.TrimSuffix(fqdn, ".")
	if !strings.HasPrefix(fqdn, "_") {
		return "", "", "", false
	}
	s := fqdn[1:]
	if i := strings.Index(s, "._tcp."); i >= 0 {
		return s[:i], "tcp", strings.TrimSuffix(s[i+len("._tcp."):], "."), true
	}
	if i := strings.Index(s, "._udp."); i >= 0 {
		return s[:i], "udp", strings.TrimSuffix(s[i+len("._udp."):], "."), true
	}
	return "", "", "", false
}

// LookupSRVTargets resolves SRV and returns sorted host:port targets (priority asc, weight desc).
func LookupSRVTargets(lookup SRVLookup, srvQuery string) ([]string, error) {
	srvQuery = strings.TrimSpace(srvQuery)
	if srvQuery == "" {
		return nil, nil
	}
	if lookup == nil {
		lookup = net.LookupSRV
	}
	svc, proto, zone, ok := ParseSRVQuery(srvQuery)
	if !ok || zone == "" {
		return nil, nil
	}
	_, addrs, err := lookup(svc, proto, zone)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(addrs, func(i, j int) bool {
		if addrs[i].Priority != addrs[j].Priority {
			return addrs[i].Priority < addrs[j].Priority
		}
		return addrs[i].Weight > addrs[j].Weight
	})
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		h := strings.TrimSuffix(strings.TrimSpace(a.Target), ".")
		if h == "" {
			continue
		}
		port := int(a.Port)
		if port <= 0 || port > 65535 {
			continue
		}
		out = append(out, net.JoinHostPort(h, strconv.Itoa(port)))
	}
	return out, nil
}

// MergePeerAddrs returns static ∪ extra, deduped; order is static first, then new from extra.
func MergePeerAddrs(static, extra []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, list := range [][]string{static, extra} {
		for _, a := range list {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			if _, ok := seen[a]; ok {
				continue
			}
			seen[a] = struct{}{}
			out = append(out, a)
		}
	}
	return out
}
