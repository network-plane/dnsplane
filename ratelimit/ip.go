// Package ratelimit provides simple per-IP token-bucket rate limiting.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package ratelimit

import (
	"net"
	"sync"
	"time"
)

type entry struct {
	tokens float64
	last   time.Time
}

// PerIP implements a token bucket per client IP (disabled when RPS <= 0).
type PerIP struct {
	mu     sync.Mutex
	m      map[string]*entry
	rps    float64
	burst  int
	maxIPs int
}

// NewPerIP creates a limiter. rps is sustained requests per second; burst is bucket size. maxIPs caps map growth (0 = 10000).
func NewPerIP(rps float64, burst int) *PerIP {
	if burst < 1 {
		burst = 1
	}
	return &PerIP{
		m:      make(map[string]*entry),
		rps:    rps,
		burst:  burst,
		maxIPs: 10000,
	}
}

// Allow reports whether one request from ip should proceed. Always true when disabled.
func (p *PerIP) Allow(ip string) bool {
	if p == nil || p.rps <= 0 {
		return true
	}
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	if ip == "" {
		ip = "unknown"
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.m) >= p.maxIPs && p.m[ip] == nil {
		// avoid unbounded memory; fail open for new IPs when table is full
		return true
	}
	e := p.m[ip]
	if e == nil {
		e = &entry{tokens: float64(p.burst), last: now}
		p.m[ip] = e
	}
	elapsed := now.Sub(e.last).Seconds()
	e.last = now
	e.tokens += elapsed * p.rps
	maxTok := float64(p.burst)
	if e.tokens > maxTok {
		e.tokens = maxTok
	}
	if e.tokens < 1 {
		return false
	}
	e.tokens--
	return true
}
