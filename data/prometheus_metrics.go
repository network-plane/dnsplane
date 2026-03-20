// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"fmt"
	"io"
	"strings"
)

// Prometheus histogram bucket upper bounds (seconds), matching perfHistLabels ranges in ms.
var prometheusResolveBucketLe = []string{
	"0.001", "0.002", "0.004", "0.008", "0.016", "0.032", "0.064", "+Inf",
}

// cumulativeFromBuckets converts exclusive bucket counts to cumulative counts for Prometheus.
func cumulativeFromBuckets(counts [8]uint64) [8]uint64 {
	var cum [8]uint64
	cum[0] = counts[0]
	for i := 1; i < 8; i++ {
		cum[i] = cum[i-1] + counts[i]
	}
	return cum
}

// WriteResolverPerfPrometheus appends Prometheus histogram lines for fast-path resolve latency
// (from RecordResolverAResolve). Emits aggregate series and per-qtype labeled histograms.
func WriteResolverPerfPrometheus(w io.Writer) {
	_, _ = fmt.Fprintf(w, "# HELP dnsplane_dns_resolve_duration_seconds DNS fast-path resolve latency (seconds)\n")
	_, _ = fmt.Fprintf(w, "# TYPE dnsplane_dns_resolve_duration_seconds histogram\n")

	n := perfATotal.Load()
	var totalCounts [8]uint64
	for i := range perfHistTotal {
		totalCounts[i] = perfHistTotal[i].Load()
	}
	sumSec := float64(perfSumTotalNs.Load()) / 1e9
	writeHistogramSeries(w, "", totalCounts, sumSec, n)

	perfQT.Range(func(k, v any) bool {
		qtype, ok := k.(string)
		if !ok || qtype == "" {
			return true
		}
		qt := v.(*perfQTStats)
		nt := qt.total.Load()
		if nt == 0 {
			return true
		}
		var qc [8]uint64
		for i := range qt.hist {
			qc[i] = qt.hist[i].Load()
		}
		sumQ := float64(qt.sumTotalNs.Load()) / 1e9
		esc := prometheusEscapeLabelValue(qtype)
		writeHistogramSeries(w, `qtype="`+esc+`"`, qc, sumQ, nt)
		return true
	})
}

func writeHistogramSeries(w io.Writer, labels string, counts [8]uint64, sum float64, count uint64) {
	cum := cumulativeFromBuckets(counts)
	for i := 0; i < 8; i++ {
		le := prometheusResolveBucketLe[i]
		if labels == "" {
			_, _ = fmt.Fprintf(w, `dnsplane_dns_resolve_duration_seconds_bucket{le="%s"} %d`+"\n", le, cum[i])
		} else {
			_, _ = fmt.Fprintf(w, `dnsplane_dns_resolve_duration_seconds_bucket{%s,le="%s"} %d`+"\n", labels, le, cum[i])
		}
	}
	if labels == "" {
		_, _ = fmt.Fprintf(w, "dnsplane_dns_resolve_duration_seconds_sum %g\n", sum)
		_, _ = fmt.Fprintf(w, "dnsplane_dns_resolve_duration_seconds_count %d\n", count)
	} else {
		_, _ = fmt.Fprintf(w, "dnsplane_dns_resolve_duration_seconds_sum{%s} %g\n", labels, sum)
		_, _ = fmt.Fprintf(w, "dnsplane_dns_resolve_duration_seconds_count{%s} %d\n", labels, count)
	}
}

func prometheusEscapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
