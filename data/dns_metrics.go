// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package data

import (
	"io"
	"strconv"
	"sync/atomic"
)

var (
	dnssecOutcomeOff      atomic.Uint64
	dnssecOutcomeInsecure atomic.Uint64
	dnssecOutcomeVerified atomic.Uint64
	dnssecOutcomeBogus    atomic.Uint64
	dnssecOutcomeSkipped  atomic.Uint64
	limiterDropQueryRate  atomic.Uint64
	limiterDropSliding    atomic.Uint64
	limiterDropRRL        atomic.Uint64
)

// RecordDNSSECOutcome increments counters for dnssecvalidate outcomes.
func RecordDNSSECOutcome(outcome string) {
	switch outcome {
	case "off":
		dnssecOutcomeOff.Add(1)
	case "insecure":
		dnssecOutcomeInsecure.Add(1)
	case "verified":
		dnssecOutcomeVerified.Add(1)
	case "bogus":
		dnssecOutcomeBogus.Add(1)
	case "skipped_local":
		dnssecOutcomeSkipped.Add(1)
	default:
		dnssecOutcomeInsecure.Add(1)
	}
}

// RecordLimiterDrop records a refused query due to response limiters.
func RecordLimiterDrop(reason string) {
	switch reason {
	case "query_rate":
		limiterDropQueryRate.Add(1)
	case "response_sliding":
		limiterDropSliding.Add(1)
	case "response_rrl":
		limiterDropRRL.Add(1)
	}
}

// WriteDNSAbusePrometheus writes counters for DNSSEC and limiter drops.
func WriteDNSAbusePrometheus(w io.Writer) {
	_, _ = io.WriteString(w, "# HELP dnsplane_dnssec_outcomes_total DNSSEC validation outcomes\n")
	_, _ = io.WriteString(w, "# TYPE dnsplane_dnssec_outcomes_total counter\n")
	writeLabeled := func(label string, v uint64) {
		_, _ = io.WriteString(w, `dnsplane_dnssec_outcomes_total{outcome="`+label+`"} `+strconv.FormatUint(v, 10)+"\n")
	}
	writeLabeled("off", dnssecOutcomeOff.Load())
	writeLabeled("insecure", dnssecOutcomeInsecure.Load())
	writeLabeled("verified", dnssecOutcomeVerified.Load())
	writeLabeled("bogus", dnssecOutcomeBogus.Load())
	writeLabeled("skipped_local", dnssecOutcomeSkipped.Load())

	_, _ = io.WriteString(w, "# HELP dnsplane_dns_limiter_drops_total Queries refused by limiter type\n")
	_, _ = io.WriteString(w, "# TYPE dnsplane_dns_limiter_drops_total counter\n")
	_, _ = io.WriteString(w, `dnsplane_dns_limiter_drops_total{reason="query_rate"} `+strconv.FormatUint(limiterDropQueryRate.Load(), 10)+"\n")
	_, _ = io.WriteString(w, `dnsplane_dns_limiter_drops_total{reason="response_sliding"} `+strconv.FormatUint(limiterDropSliding.Load(), 10)+"\n")
	_, _ = io.WriteString(w, `dnsplane_dns_limiter_drops_total{reason="response_rrl"} `+strconv.FormatUint(limiterDropRRL.Load(), 10)+"\n")
}
