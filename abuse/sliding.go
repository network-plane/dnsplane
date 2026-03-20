// Package abuse implements response-side rate controls (sliding window, RRL).
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package abuse

import (
	"sync"
	"time"
)

// SlidingWindow limits responses per client IP per fixed time window.
type SlidingWindow struct {
	mu       sync.Mutex
	m        map[string]*swEntry
	window   time.Duration
	maxPerIP int
	maxIPs   int
	now      func() time.Time
}

type swEntry struct {
	count int
	start time.Time
}

// NewSlidingWindow creates a limiter. maxPerIP is max responses per IP per window; maxIPs caps map size (0 = 50000).
func NewSlidingWindow(window time.Duration, maxPerIP int, maxIPs int) *SlidingWindow {
	if maxIPs <= 0 {
		maxIPs = 50000
	}
	return &SlidingWindow{
		m:        make(map[string]*swEntry),
		window:   window,
		maxPerIP: maxPerIP,
		maxIPs:   maxIPs,
		now:      time.Now,
	}
}

// Allow reports whether this response may proceed (counts against cap when RecordResponse is called).
func (s *SlidingWindow) Allow(ip, _ string) bool {
	if s == nil || s.maxPerIP <= 0 {
		return true
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.m) >= s.maxIPs && s.m[ip] == nil {
		return true // fail open
	}
	e := s.m[ip]
	if e == nil {
		return true
	}
	if now.Sub(e.start) >= s.window {
		return true
	}
	return e.count < s.maxPerIP
}

// RecordResponse increments the counter for ip after a response is sent.
func (s *SlidingWindow) RecordResponse(ip, _ string) {
	if s == nil || s.maxPerIP <= 0 {
		return
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[ip]
	if e == nil {
		s.m[ip] = &swEntry{count: 1, start: now}
		return
	}
	if now.Sub(e.start) >= s.window {
		e.count = 1
		e.start = now
		return
	}
	e.count++
}
