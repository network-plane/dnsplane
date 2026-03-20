// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package abuse

import (
	"hash/fnv"
	"math/rand"
	"sync"
	"time"
)

// RRL approximates response rate limiting per (ip, qname) with probabilistic slip when over cap.
type RRL struct {
	mu         sync.Mutex
	buckets    map[uint64]*rrlEntry
	maxPerWin  int           // max responses per window per bucket
	window     time.Duration // e.g. 1s
	slip       float64       // probability to allow when over (0-1)
	maxBuckets int
	now        func() time.Time
	rnd        *rand.Rand
}

type rrlEntry struct {
	count int
	start time.Time
}

// NewRRL creates an RRL limiter: at most maxPerWindow responses per (ip,qname) per window; slip allows some through when over.
func NewRRL(maxPerWindow int, window time.Duration, slip float64, maxBuckets int) *RRL {
	if maxBuckets <= 0 {
		maxBuckets = 100000
	}
	return &RRL{
		buckets:    make(map[uint64]*rrlEntry),
		maxPerWin:  maxPerWindow,
		window:     window,
		slip:       slip,
		maxBuckets: maxBuckets,
		now:        time.Now,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func rrlKey(ip, qname string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(ip))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(qname))
	return h.Sum64()
}

// Allow returns false if this response should be refused (rate limited), unless slip passes.
func (r *RRL) Allow(ip, qname string) bool {
	if r == nil || r.maxPerWin <= 0 {
		return true
	}
	now := r.now()
	k := rrlKey(ip, qname)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buckets) >= r.maxBuckets && r.buckets[k] == nil {
		return true
	}
	e := r.buckets[k]
	if e == nil {
		return true
	}
	if now.Sub(e.start) >= r.window {
		return true
	}
	if e.count < r.maxPerWin {
		return true
	}
	if r.slip > 0 && r.rnd.Float64() < r.slip {
		return true
	}
	return false
}

// RecordResponse increments the bucket after a response is emitted.
func (r *RRL) RecordResponse(ip, qname string) {
	if r == nil || r.maxPerWin <= 0 {
		return
	}
	now := r.now()
	k := rrlKey(ip, qname)
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.buckets[k]
	if e == nil {
		r.buckets[k] = &rrlEntry{count: 1, start: now}
		return
	}
	if now.Sub(e.start) >= r.window {
		e.count = 1
		e.start = now
		return
	}
	e.count++
}
