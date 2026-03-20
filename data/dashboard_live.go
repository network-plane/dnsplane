// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"sync"
	"time"
)

// Live dashboard ring buffer and per-minute aggregates (in-memory only).
const (
	dashboardLogCap      = 400
	dashboardSeriesSlots = 60 // last 60 minutes
)

// DashboardResolution is one resolved query for the rolling log API.
type DashboardResolution struct {
	At         time.Time `json:"at"`
	Qname      string    `json:"qname"`
	Qtype      string    `json:"qtype"`
	Outcome    string    `json:"outcome"`
	Upstream   string    `json:"upstream,omitempty"`
	Record     string    `json:"record"`
	DurationMs float64   `json:"duration_ms"`
}

// DashboardMinutePoint is one minute bucket for charts (replies count + avg latency).
type DashboardMinutePoint struct {
	T       string  `json:"t"`
	Replies uint64  `json:"replies"`
	AvgMs   float64 `json:"avg_ms"`
}

type minuteAgg struct {
	count uint64
	sumMs float64
}

var dashboardLive struct {
	mu     sync.Mutex
	log    []DashboardResolution
	minute map[int64]*minuteAgg
}

// RecordDashboardResolution appends to the rolling log and updates per-minute stats.
func RecordDashboardResolution(e DashboardResolution) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	} else {
		e.At = e.At.UTC()
	}

	dashboardLive.mu.Lock()
	defer dashboardLive.mu.Unlock()

	if dashboardLive.minute == nil {
		dashboardLive.minute = make(map[int64]*minuteAgg)
	}

	// Log ring (keep last N)
	dashboardLive.log = append(dashboardLive.log, e)
	if len(dashboardLive.log) > dashboardLogCap {
		dashboardLive.log = dashboardLive.log[len(dashboardLive.log)-dashboardLogCap:]
	}

	mk := e.At.Unix() / 60
	agg := dashboardLive.minute[mk]
	if agg == nil {
		agg = &minuteAgg{}
		dashboardLive.minute[mk] = agg
	}
	agg.count++
	agg.sumMs += e.DurationMs

	// Prune buckets older than dashboardSeriesSlots minutes
	cutoff := mk - int64(dashboardSeriesSlots) - 1
	for k := range dashboardLive.minute {
		if k < cutoff {
			delete(dashboardLive.minute, k)
		}
	}
}

// GetDashboardLogNewestFirst returns up to limit entries, newest first.
func GetDashboardLogNewestFirst(limit int) []DashboardResolution {
	if limit <= 0 {
		limit = 100
	}
	if limit > dashboardLogCap {
		limit = dashboardLogCap
	}
	dashboardLive.mu.Lock()
	defer dashboardLive.mu.Unlock()
	n := len(dashboardLive.log)
	if n == 0 {
		return nil
	}
	if limit > n {
		limit = n
	}
	out := make([]DashboardResolution, 0, limit)
	for i := n - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, dashboardLive.log[i])
	}
	return out
}

// GetDashboardSeries returns the last dashboardSeriesSlots minutes, oldest → newest.
func GetDashboardSeries() []DashboardMinutePoint {
	dashboardLive.mu.Lock()
	defer dashboardLive.mu.Unlock()

	nowMin := time.Now().UTC().Unix() / 60
	out := make([]DashboardMinutePoint, 0, dashboardSeriesSlots)
	for i := int64(0); i < int64(dashboardSeriesSlots); i++ {
		mk := nowMin - int64(dashboardSeriesSlots) + 1 + i
		t := time.Unix(mk*60, 0).UTC()
		pt := DashboardMinutePoint{T: t.Format(time.RFC3339)}
		if agg := dashboardLive.minute[mk]; agg != nil {
			pt.Replies = agg.count
			if agg.count > 0 {
				pt.AvgMs = agg.sumMs / float64(agg.count)
			}
		}
		out = append(out, pt)
	}
	return out
}
