// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Live dashboard ring buffer and per-minute aggregates (in-memory only).
const (
	defaultDashboardResolutionLogCap = 1000
	maxDashboardResolutionLogCap     = 1000000 // keep in sync with config.applyDefaults clamp
	dashboardSeriesSlots             = 60      // last 60 minutes
	dashboardSecondRetain            = 65      // per-second buckets kept for pruning headroom
)

var dashboardResolutionLogCap atomic.Int32 // 0 until SetDashboardResolutionLogCap (DashboardLogCap falls back to default)

// SetDashboardResolutionLogCap sets the in-memory ring size for dashboard resolution rows (from config).
// Values outside [1, maxDashboardResolutionLogCap] are clamped; existing entries are truncated if the cap shrinks.
func SetDashboardResolutionLogCap(n int) {
	if n <= 0 {
		n = defaultDashboardResolutionLogCap
	}
	if n > maxDashboardResolutionLogCap {
		n = maxDashboardResolutionLogCap
	}
	dashboardResolutionLogCap.Store(int32(n))
	dashboardLive.mu.Lock()
	defer dashboardLive.mu.Unlock()
	if len(dashboardLive.log) > n {
		dashboardLive.log = dashboardLive.log[len(dashboardLive.log)-n:]
	}
}

// DashboardLogCap is the maximum number of resolution rows kept in memory for the dashboard.
func DashboardLogCap() int {
	v := int(dashboardResolutionLogCap.Load())
	if v <= 0 {
		return defaultDashboardResolutionLogCap
	}
	return v
}

// DashboardResolution is one resolved query for the rolling log API.
type DashboardResolution struct {
	At         time.Time `json:"at"`
	ClientIP   string    `json:"client_ip,omitempty"`
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

// secondAgg holds resolution counts for one UTC second (from QueryObserver / RecordDashboardResolution).
type secondAgg struct {
	total    uint64
	local    uint64
	cache    uint64
	upstream uint64
	none     uint64
	blocked  uint64
}

var dashboardLive struct {
	mu     sync.Mutex
	log    []DashboardResolution
	minute map[int64]*minuteAgg
	second map[int64]*secondAgg
}

// DashboardPerSecRates is average resolutions per second over a short window (see WindowSeconds).
type DashboardPerSecRates struct {
	WindowSeconds int     `json:"window_seconds"`
	Resolutions   float64 `json:"resolutions_per_sec"`
	Local         float64 `json:"local_per_sec"`
	Cache         float64 `json:"cache_per_sec"`
	Upstream      float64 `json:"upstream_per_sec"`
	None          float64 `json:"none_per_sec"`
	Blocked       float64 `json:"blocked_per_sec"`
}

// GetDashboardPerSecRates returns average rates over the last windowSeconds UTC seconds (excluding the current partial second).
// windowSeconds is clamped to [1, 60]; default used when out of range is 5.
func GetDashboardPerSecRates(windowSeconds int) DashboardPerSecRates {
	if windowSeconds < 1 {
		windowSeconds = 5
	}
	if windowSeconds > 60 {
		windowSeconds = 60
	}
	now := time.Now().UTC().Unix()
	var tot, loc, cac, up, non, blk uint64
	dashboardLive.mu.Lock()
	for i := int64(0); i < int64(windowSeconds); i++ {
		sk := now - 1 - i
		if sb := dashboardLive.second[sk]; sb != nil {
			tot += sb.total
			loc += sb.local
			cac += sb.cache
			up += sb.upstream
			non += sb.none
			blk += sb.blocked
		}
	}
	dashboardLive.mu.Unlock()
	wf := float64(windowSeconds)
	return DashboardPerSecRates{
		WindowSeconds: windowSeconds,
		Resolutions:   float64(tot) / wf,
		Local:         float64(loc) / wf,
		Cache:         float64(cac) / wf,
		Upstream:      float64(up) / wf,
		None:          float64(non) / wf,
		Blocked:       float64(blk) / wf,
	}
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
	if dashboardLive.second == nil {
		dashboardLive.second = make(map[int64]*secondAgg)
	}
	sk := e.At.Unix()
	sb := dashboardLive.second[sk]
	if sb == nil {
		sb = &secondAgg{}
		dashboardLive.second[sk] = sb
	}
	sb.total++
	switch strings.ToLower(strings.TrimSpace(e.Outcome)) {
	case "local":
		sb.local++
	case "cache":
		sb.cache++
	case "upstream":
		sb.upstream++
	case "none":
		sb.none++
	case "blocked":
		sb.blocked++
	}
	cutS := sk - int64(dashboardSecondRetain)
	for k := range dashboardLive.second {
		if k < cutS {
			delete(dashboardLive.second, k)
		}
	}

	// Log ring (keep last N)
	capVal := DashboardLogCap()
	dashboardLive.log = append(dashboardLive.log, e)
	if len(dashboardLive.log) > capVal {
		dashboardLive.log = dashboardLive.log[len(dashboardLive.log)-capVal:]
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
	capVal := DashboardLogCap()
	if limit > capVal {
		limit = capVal
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

// ClearDashboardResolutionLog removes all entries from the in-memory resolution ring.
// Per-minute chart aggregates (GetDashboardSeries) are unchanged.
func ClearDashboardResolutionLog() {
	dashboardLive.mu.Lock()
	defer dashboardLive.mu.Unlock()
	dashboardLive.log = nil
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
