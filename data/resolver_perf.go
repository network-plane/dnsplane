// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"sync"
	"sync/atomic"
	"time"
)

// A-query resolve outcomes (IPv4 parallel path in resolver).
const (
	PerfOutcomeLocal = iota
	PerfOutcomeCache
	PerfOutcomeUpstream
	PerfOutcomeNone
)

var (
	perfMu sync.Mutex

	perfATotal          atomic.Uint64
	perfOutcomeLocal    atomic.Uint64
	perfOutcomeCache    atomic.Uint64
	perfOutcomeUpstream atomic.Uint64
	perfOutcomeNone     atomic.Uint64

	perfSumTotalNs        atomic.Uint64
	perfSumPrepNs         atomic.Uint64
	perfSumUpstreamWaitNs atomic.Uint64 // max(upstream) - prep when outcome upstream; else 0
	perfSumMaxUpstreamNs  atomic.Uint64 // slowest upstream completion (since t0), upstream path only
	perfMaxTotalNs        atomic.Uint64

	// Histogram of total resolve time (nanoseconds), 8 buckets (ms ranges).
	perfHistTotal [8]atomic.Uint64
	// Histogram for upstream-path resolves only.
	perfHistUpstream [8]atomic.Uint64

	perfUpstreamCountSum atomic.Uint64 // sum of len(serversToQuery) per upstream outcome
	perfFirstRecord      atomic.Int64  // unix nano of first A-resolve (for uptime of perf window)
)

func perfBucketIndex(totalNs uint64) int {
	ms := totalNs / 1e6
	switch {
	case ms < 1:
		return 0
	case ms < 2:
		return 1
	case ms < 4:
		return 2
	case ms < 8:
		return 3
	case ms < 16:
		return 4
	case ms < 32:
		return 5
	case ms < 64:
		return 6
	default:
		return 7
	}
}

// RecordResolverAResolve records one A-record parallel resolve (timing from resolver).
// prepNs: max(elapsed local goroutine, elapsed cache goroutine) when both are known; on local-only
// win before cache completes, pass localElapsed only.
// maxUpstreamNs: slowest upstream completion time since t0 (0 if no upstream).
// upstreamWaitNs: for upstream outcome, time from prep done until all upstreams done (totalNs - prepNs
// when upstream-bound); 0 otherwise.
// upstreamServers: len(serversToQuery) for upstream outcome.
func RecordResolverAResolve(outcome int, totalNs, prepNs, maxUpstreamNs, upstreamWaitNs uint64, upstreamServers int) {
	if perfFirstRecord.Load() == 0 {
		perfFirstRecord.CompareAndSwap(0, time.Now().UnixNano())
	}

	perfATotal.Add(1)
	switch outcome {
	case PerfOutcomeLocal:
		perfOutcomeLocal.Add(1)
	case PerfOutcomeCache:
		perfOutcomeCache.Add(1)
	case PerfOutcomeUpstream:
		perfOutcomeUpstream.Add(1)
		perfSumMaxUpstreamNs.Add(maxUpstreamNs)
		perfSumUpstreamWaitNs.Add(upstreamWaitNs)
		perfUpstreamCountSum.Add(uint64(upstreamServers))
	case PerfOutcomeNone:
		perfOutcomeNone.Add(1)
	}

	perfSumTotalNs.Add(totalNs)
	perfSumPrepNs.Add(prepNs)

	b := perfBucketIndex(totalNs)
	perfHistTotal[b].Add(1)
	if outcome == PerfOutcomeUpstream {
		perfHistUpstream[b].Add(1)
	}

	for {
		old := perfMaxTotalNs.Load()
		if totalNs <= old || perfMaxTotalNs.CompareAndSwap(old, totalNs) {
			break
		}
	}
}

// ResetResolverPerf zeros all A-resolve performance counters (e.g. before a benchmark).
func ResetResolverPerf() {
	perfMu.Lock()
	defer perfMu.Unlock()

	perfATotal.Store(0)
	perfOutcomeLocal.Store(0)
	perfOutcomeCache.Store(0)
	perfOutcomeUpstream.Store(0)
	perfOutcomeNone.Store(0)
	perfSumTotalNs.Store(0)
	perfSumPrepNs.Store(0)
	perfSumUpstreamWaitNs.Store(0)
	perfSumMaxUpstreamNs.Store(0)
	perfMaxTotalNs.Store(0)
	perfUpstreamCountSum.Store(0)
	perfFirstRecord.Store(0)
	for i := range perfHistTotal {
		perfHistTotal[i].Store(0)
		perfHistUpstream[i].Store(0)
	}
}

// ResolverPerfReport is JSON for GET /stats/perf.
type ResolverPerfReport struct {
	Description string `json:"description"`
	SinceFirst  string `json:"since_first_resolve,omitempty"`

	AResolve struct {
		Total              uint64  `json:"total"`
		OutcomeLocal       uint64  `json:"outcome_local"`
		OutcomeCache       uint64  `json:"outcome_cache"`
		OutcomeUpstream    uint64  `json:"outcome_upstream"`
		OutcomeNone        uint64  `json:"outcome_none"`
		AvgTotalMs         float64 `json:"avg_total_ms"`
		AvgPrepMs          float64 `json:"avg_prep_ms"`
		MaxTotalMs         float64 `json:"max_total_ms"`
		AvgUpstreamWaitMs  float64 `json:"avg_upstream_wait_ms,omitempty"` // upstream outcomes only
		AvgMaxUpstreamMs   float64 `json:"avg_max_upstream_ms,omitempty"`
		AvgUpstreamServers float64 `json:"avg_upstream_servers,omitempty"`
		HistTotalMs        []struct {
			Label string `json:"label"`
			Count uint64 `json:"count"`
		} `json:"histogram_total_ms"`
		HistUpstreamMs []struct {
			Label string `json:"label"`
			Count uint64 `json:"count"`
		} `json:"histogram_upstream_ms"`
	} `json:"a_resolve"`

	Notes []string `json:"notes"`
}

var perfHistLabels = []string{
	"[0,1)", "[1,2)", "[2,4)", "[4,8)", "[8,16)", "[16,32)", "[32,64)", "[64,inf)",
}

// GetResolverPerfReport builds a snapshot for the API.
func GetResolverPerfReport() ResolverPerfReport {
	var r ResolverPerfReport
	r.Description = "A-record parallel resolve timings (local+cache goroutines vs upstream wait). prep = max(local_elapsed, cache_elapsed) from query start."
	r.Notes = []string{
		"prep_ms: time until both local and cache lookups finished (parallel).",
		"Upstream path waits for every configured upstream; total ≈ max(prep, slowest_upstream).",
		"POST /stats/perf/reset clears counters.",
	}

	if ts := perfFirstRecord.Load(); ts > 0 {
		r.SinceFirst = time.Unix(0, ts).UTC().Format(time.RFC3339Nano)
	}

	n := perfATotal.Load()
	r.AResolve.Total = n
	r.AResolve.OutcomeLocal = perfOutcomeLocal.Load()
	r.AResolve.OutcomeCache = perfOutcomeCache.Load()
	r.AResolve.OutcomeUpstream = perfOutcomeUpstream.Load()
	r.AResolve.OutcomeNone = perfOutcomeNone.Load()

	if n > 0 {
		r.AResolve.AvgTotalMs = float64(perfSumTotalNs.Load()) / float64(n) / 1e6
		r.AResolve.AvgPrepMs = float64(perfSumPrepNs.Load()) / float64(n) / 1e6
	}
	r.AResolve.MaxTotalMs = float64(perfMaxTotalNs.Load()) / 1e6

	nu := perfOutcomeUpstream.Load()
	if nu > 0 {
		r.AResolve.AvgUpstreamWaitMs = float64(perfSumUpstreamWaitNs.Load()) / float64(nu) / 1e6
		r.AResolve.AvgMaxUpstreamMs = float64(perfSumMaxUpstreamNs.Load()) / float64(nu) / 1e6
		r.AResolve.AvgUpstreamServers = float64(perfUpstreamCountSum.Load()) / float64(nu)
	}

	for i := 0; i < 8; i++ {
		r.AResolve.HistTotalMs = append(r.AResolve.HistTotalMs, struct {
			Label string `json:"label"`
			Count uint64 `json:"count"`
		}{Label: perfHistLabels[i], Count: perfHistTotal[i].Load()})
		r.AResolve.HistUpstreamMs = append(r.AResolve.HistUpstreamMs, struct {
			Label string `json:"label"`
			Count uint64 `json:"count"`
		}{Label: perfHistLabels[i], Count: perfHistUpstream[i].Load()})
	}

	return r
}
