// Package dnssecvalidate performs best-effort DNSSEC signature checks on upstream responses.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnssecvalidate

import (
	"strings"
	"time"

	"dnsplane/config"

	"github.com/miekg/dns"

	"dnsplane/safecast"
)

// Outcome values for metrics/logging.
const (
	OutcomeOff      = "off"
	OutcomeInsecure = "insecure"
	OutcomeVerified = "verified"
	OutcomeBogus    = "bogus"
	OutcomeSkipped  = "skipped_local"
)

// ApplyToUpstreamAnswer validates RRSIGs when DNSKEYs are present in the message.
// It does not perform full chain validation to the root (iterative resolution).
// Returns outcome and whether the caller should SERVFAIL (bogus + strict).
func ApplyToUpstreamAnswer(req *dns.Msg, answer *dns.Msg, _ dns.Question, cfg config.Config) (outcome string, servfail bool) {
	if !cfg.DNSSECValidate {
		return OutcomeOff, false
	}
	if answer == nil {
		return OutcomeInsecure, false
	}
	// Strip upstream AD — we only set AD after local verification.
	answer.AuthenticatedData = false

	rrsigs := collectRRSIGs(answer.Answer)
	if len(rrsigs) == 0 && len(collectRRSIGs(answer.Ns)) > 0 {
		rrsigs = collectRRSIGs(answer.Ns)
	}
	if len(rrsigs) == 0 {
		return OutcomeInsecure, false
	}

	keys := collectDNSKEYs(answer.Answer, answer.Ns, answer.Extra)
	if len(keys) == 0 {
		if cfg.DNSSECValidateStrict {
			return OutcomeBogus, true
		}
		return OutcomeInsecure, false
	}

	now := time.Now()
	ok := true
	for _, sig := range rrsigs {
		if !rrsigTimeValid(sig, now) {
			ok = false
			break
		}
		rrset := rrsetForSignature(answer, sig)
		if len(rrset) == 0 {
			ok = false
			break
		}
		k := findDNSKEY(keys, sig)
		if k == nil {
			ok = false
			break
		}
		if err := sig.Verify(k, rrset); err != nil {
			ok = false
			break
		}
	}
	if !ok {
		return OutcomeBogus, cfg.DNSSECValidateStrict
	}

	// Set AD only if client sent DO (RFC 4035).
	if req != nil && req.IsEdns0() != nil && req.IsEdns0().Do() {
		answer.AuthenticatedData = true
	}
	return OutcomeVerified, false
}

func rrsigTimeValid(sig *dns.RRSIG, now time.Time) bool {
	if sig == nil {
		return false
	}
	now32 := safecast.UnixSecondsToDNSUint32(now.Unix())
	return sig.Inception <= now32 && now32 <= sig.Expiration
}

func collectRRSIGs(rrs []dns.RR) []*dns.RRSIG {
	var out []*dns.RRSIG
	for _, rr := range rrs {
		if s, ok := rr.(*dns.RRSIG); ok {
			out = append(out, s)
		}
	}
	return out
}

func collectDNSKEYs(rrs ...[]dns.RR) []*dns.DNSKEY {
	var out []*dns.DNSKEY
	for _, block := range rrs {
		for _, rr := range block {
			if k, ok := rr.(*dns.DNSKEY); ok {
				out = append(out, k)
			}
		}
	}
	return out
}

func findDNSKEY(keys []*dns.DNSKEY, sig *dns.RRSIG) *dns.DNSKEY {
	for _, k := range keys {
		if k.KeyTag() == sig.KeyTag && strings.EqualFold(k.Hdr.Name, sig.SignerName) {
			return k
		}
	}
	return nil
}

func rrsetForSignature(msg *dns.Msg, sig *dns.RRSIG) []dns.RR {
	// Find RRs matching TypeCovered and owner name from same section as sig.
	name := sig.Hdr.Name
	typ := sig.TypeCovered
	var block []dns.RR
	switch {
	case containsRRSIG(msg.Answer, sig):
		block = msg.Answer
	case containsRRSIG(msg.Ns, sig):
		block = msg.Ns
	default:
		block = msg.Answer
	}
	var out []dns.RR
	for _, rr := range block {
		h := rr.Header()
		if h.Rrtype == dns.TypeRRSIG {
			continue
		}
		if h.Name == name && h.Rrtype == typ {
			out = append(out, rr)
		}
	}
	return out
}

func containsRRSIG(rrs []dns.RR, sig *dns.RRSIG) bool {
	for _, rr := range rrs {
		if rr == sig {
			return true
		}
	}
	return false
}
