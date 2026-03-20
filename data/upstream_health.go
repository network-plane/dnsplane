// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"sync"
	"time"

	"dnsplane/config"
	"dnsplane/dnsservers"
)

// UpstreamHealthTracker tracks per-upstream probe/forward outcomes when health checks are enabled.
type UpstreamHealthTracker struct {
	mu sync.RWMutex
	by map[string]*upstreamHealthEntry
}

type upstreamHealthEntry struct {
	unhealthy bool
	failures  int
	lastProbe time.Time
	lastErr   string
	lastOK    time.Time
}

// NewUpstreamHealthTracker creates an empty tracker.
func NewUpstreamHealthTracker() *UpstreamHealthTracker {
	return &UpstreamHealthTracker{by: make(map[string]*upstreamHealthEntry)}
}

func (t *UpstreamHealthTracker) ensure(key string) *upstreamHealthEntry {
	if t.by[key] == nil {
		t.by[key] = &upstreamHealthEntry{}
	}
	return t.by[key]
}

// Filter returns addresses to use for forwarding. Unhealthy servers are omitted when checks are enabled.
// If every address would be removed, returns the original list (degenerate fallback) so DNS still works.
func (t *UpstreamHealthTracker) Filter(cfg *config.Config, addrs []string) []string {
	if t == nil || cfg == nil || !cfg.UpstreamHealthCheckEnabled || len(addrs) == 0 {
		return addrs
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		e := t.by[a]
		if e == nil || !e.unhealthy {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return addrs
	}
	return out
}

// RecordForwardSuccess marks the upstream healthy after a successful client query.
func (t *UpstreamHealthTracker) RecordForwardSuccess(key string) {
	if t == nil || key == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.ensure(key)
	e.unhealthy = false
	e.failures = 0
	e.lastErr = ""
	e.lastOK = time.Now()
}

// ProbeOK records a successful health probe.
func (t *UpstreamHealthTracker) ProbeOK(key string) {
	t.RecordForwardSuccess(key)
}

// ProbeFail increments failures and marks unhealthy after threshold. Returns true if transitioned to unhealthy.
func (t *UpstreamHealthTracker) ProbeFail(key string, errStr string, threshold int) (nowUnhealthy bool) {
	if t == nil || key == "" {
		return false
	}
	if threshold <= 0 {
		threshold = 3
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.ensure(key)
	was := e.unhealthy
	e.failures++
	e.lastProbe = time.Now()
	e.lastErr = errStr
	if e.failures >= threshold {
		e.unhealthy = true
	}
	return e.unhealthy && !was
}

// IsUnhealthy reports whether the server is currently marked down.
func (t *UpstreamHealthTracker) IsUnhealthy(key string) bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	e := t.by[key]
	return e != nil && e.unhealthy
}

// UpstreamHealthStatus is JSON for API responses.
type UpstreamHealthStatus struct {
	AddressPort         string `json:"address_port"`
	Unhealthy           bool   `json:"unhealthy"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastProbeAt         string `json:"last_probe_at,omitempty"`
	LastProbeError      string `json:"last_probe_error,omitempty"`
	LastSuccessAt       string `json:"last_success_at,omitempty"`
}

// Snapshot returns one row per configured active upstream key seen in probes or forwards.
func (t *UpstreamHealthTracker) Snapshot(keys []string) []UpstreamHealthStatus {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]UpstreamHealthStatus, 0, len(keys))
	for _, k := range keys {
		e := t.by[k]
		st := UpstreamHealthStatus{AddressPort: k}
		if e != nil {
			st.Unhealthy = e.unhealthy
			st.ConsecutiveFailures = e.failures
			if !e.lastProbe.IsZero() {
				st.LastProbeAt = e.lastProbe.UTC().Format(time.RFC3339Nano)
			}
			st.LastProbeError = e.lastErr
			if !e.lastOK.IsZero() {
				st.LastSuccessAt = e.lastOK.UTC().Format(time.RFC3339Nano)
			}
		}
		out = append(out, st)
	}
	return out
}

// FilterHealthyUpstreamEndpoints removes upstreams marked unhealthy when health checks are enabled.
func (d *DNSResolverData) FilterHealthyUpstreamEndpoints(eps []dnsservers.UpstreamEndpoint) []dnsservers.UpstreamEndpoint {
	if d == nil || len(eps) == 0 {
		return eps
	}
	d.mu.RLock()
	h := d.upstreamHealth
	cfg := d.Settings
	d.mu.RUnlock()
	if h == nil {
		return eps
	}
	keys := make([]string, len(eps))
	for i := range eps {
		keys[i] = eps[i].HealthKey()
	}
	filtered := h.Filter(&cfg, keys)
	keep := make(map[string]struct{}, len(filtered))
	for _, k := range filtered {
		keep[k] = struct{}{}
	}
	out := make([]dnsservers.UpstreamEndpoint, 0, len(filtered))
	for _, ep := range eps {
		if _, ok := keep[ep.HealthKey()]; ok {
			out = append(out, ep)
		}
	}
	if len(out) == 0 {
		return eps
	}
	return out
}

// RecordUpstreamForwardSuccess clears unhealthy state after a successful upstream reply.
func (d *DNSResolverData) RecordUpstreamForwardSuccess(addrPort string) {
	if d == nil {
		return
	}
	d.mu.RLock()
	h := d.upstreamHealth
	d.mu.RUnlock()
	if h != nil {
		h.RecordForwardSuccess(addrPort)
	}
}

// UpstreamHealthStatuses returns per-active-upstream health for the API.
func (d *DNSResolverData) UpstreamHealthStatuses() ([]UpstreamHealthStatus, bool) {
	if d == nil {
		return nil, false
	}
	d.mu.RLock()
	servers := append([]dnsservers.DNSServer(nil), d.DNSServers...)
	h := d.upstreamHealth
	enabled := d.Settings.UpstreamHealthCheckEnabled
	d.mu.RUnlock()
	var keys []string
	for _, s := range servers {
		if !s.Active {
			continue
		}
		keys = append(keys, dnsservers.ServerToEndpoint(s).HealthKey())
	}
	if h == nil {
		out := make([]UpstreamHealthStatus, len(keys))
		for i, k := range keys {
			out[i] = UpstreamHealthStatus{AddressPort: k}
		}
		return out, enabled
	}
	st := h.Snapshot(keys)
	if !enabled {
		for i := range st {
			st[i].Unhealthy = false
			st[i].ConsecutiveFailures = 0
			st[i].LastProbeError = ""
		}
	}
	return st, enabled
}

// ApplyUpstreamProbeResult records one probe outcome. warn is called when an upstream becomes unhealthy.
func (d *DNSResolverData) ApplyUpstreamProbeResult(key string, ok bool, errStr string, warn func(msg string, kv ...any)) {
	if d == nil {
		return
	}
	d.mu.RLock()
	cfg := d.Settings
	h := d.upstreamHealth
	d.mu.RUnlock()
	if h == nil || !cfg.UpstreamHealthCheckEnabled {
		return
	}
	th := cfg.UpstreamHealthCheckFailures
	if th <= 0 {
		th = 3
	}
	if ok {
		h.ProbeOK(key)
		return
	}
	if h.ProbeFail(key, errStr, th) && warn != nil {
		warn("upstream marked unhealthy after repeated probe failures", "server", key, "error", errStr, "threshold", th)
	}
}
