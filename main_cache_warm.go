// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"strings"
	"time"

	"dnsplane/data"

	"github.com/miekg/dns"
)

// runCacheWarmLoop sends a lightweight self-query periodically to keep the Go
// process hot (CPU caches, memory pages, goroutine scheduling). This prevents
// cold-start latency spikes after idle periods.
func runCacheWarmLoop(dnsData *data.DNSResolverData, dnsPort string) {
	for {
		cfg := dnsData.GetResolverSettings()
		if !cfg.CacheWarmEnabled {
			time.Sleep(30 * time.Second)
			continue
		}
		interval := time.Duration(cfg.CacheWarmIntervalSeconds) * time.Second
		if interval < 1*time.Second {
			interval = 10 * time.Second
		}
		time.Sleep(interval)

		port := strings.TrimSpace(dnsPort)
		if port == "" {
			port = "53"
		}
		m := new(dns.Msg)
		m.SetQuestion("localhost.", dns.TypeA)
		m.RecursionDesired = false
		c := new(dns.Client)
		c.Timeout = 1 * time.Second
		_, _, _ = c.Exchange(m, "127.0.0.1:"+port)
	}
}
