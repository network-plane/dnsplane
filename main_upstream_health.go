// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"dnsplane/data"
	"dnsplane/resolver"

	"github.com/miekg/dns"
)

// runUpstreamHealthProbeLoop periodically probes each active upstream when enabled in config.
func runUpstreamHealthProbeLoop(dnsData *data.DNSResolverData, dnsLogger *slog.Logger) {
	client := resolver.NewDNSClient(3 * time.Second)
	warn := func(msg string, kv ...any) {
		if dnsLogger != nil {
			dnsLogger.Warn(msg, kv...)
		}
	}
	for {
		cfg := dnsData.GetResolverSettings()
		if !cfg.UpstreamHealthCheckEnabled {
			time.Sleep(10 * time.Second)
			continue
		}
		interval := time.Duration(cfg.UpstreamHealthCheckIntervalSeconds) * time.Second
		if interval < 5*time.Second {
			interval = 30 * time.Second
		}
		qname := strings.TrimSpace(cfg.UpstreamHealthCheckQueryName)
		if qname == "" {
			qname = "google.com."
		}
		if !strings.HasSuffix(qname, ".") {
			qname += "."
		}
		servers := dnsData.GetServers()
		for _, s := range servers {
			if !s.Active {
				continue
			}
			port := strings.TrimSpace(s.Port)
			if port == "" {
				port = "53"
			}
			key := s.Address + ":" + port
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			q := dns.Question{Name: qname, Qtype: dns.TypeA, Qclass: dns.ClassINET}
			msg, err := client.Query(ctx, q, key)
			cancel()
			var ok bool
			var errStr string
			switch {
			case err != nil:
				errStr = err.Error()
			case msg == nil:
				errStr = "nil response"
			case msg.Rcode != dns.RcodeSuccess && msg.Rcode != dns.RcodeNameError:
				errStr = fmt.Sprintf("rcode %d", msg.Rcode)
			default:
				ok = true
			}
			dnsData.ApplyUpstreamProbeResult(key, ok, errStr, warn)
		}
		time.Sleep(interval)
	}
}
