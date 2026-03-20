// Package dnsserve provides a shared DNS request/response path for UDP, TCP, DoT, and DoH.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsserve

import (
	"context"
	"strings"

	"dnsplane/config"
	"dnsplane/resolver"

	"github.com/miekg/dns"
)

// Protocol identifies how the query arrived (for metrics).
const (
	ProtoUDP = "udp"
	ProtoTCP = "tcp"
	ProtoDoT = "dot"
	ProtoDoH = "doh"
)

// ServeMeta is per-request client metadata.
type ServeMeta struct {
	ClientIP string
	Protocol string
}

// Dependencies bundles runtime services for ServeDNS (avoids package cycles with main).
type Dependencies struct {
	Resolver *resolver.Resolver
	Settings func() config.Config
	// Optional response-side abuse limiter (nil = disabled).
	ResponseLimiter ResponseLimiter
	// QueryLimiter is the existing per-IP query token bucket (nil = disabled).
	QueryLimiter QueryLimiter
	// OnLimiterDrop is called when a limiter refuses (reason: query_rate, response_sliding, response_rrl).
	OnLimiterDrop func(reason string)
}

// QueryLimiter matches ratelimit.PerIP.Allow.
type QueryLimiter interface {
	Allow(ip string) bool
}

// ResponseLimiter is sliding-window or RRL (see abuse package).
type ResponseLimiter interface {
	Allow(ip, qname string) bool
	RecordResponse(ip, qname string)
}

// ServeDNS builds the response for one DNS message. Caller writes wire to the client.
func ServeDNS(ctx context.Context, req *dns.Msg, meta ServeMeta, dep Dependencies) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = false

	clientIP := strings.TrimSpace(meta.ClientIP)
	if clientIP == "" {
		clientIP = "unknown"
	}

	st := dep.Settings()
	if st.DNSRefuseANY && hasANYQuestion(req) {
		resp.SetRcode(req, dns.RcodeNotImplemented)
		return resp
	}

	if dep.QueryLimiter != nil && !dep.QueryLimiter.Allow(clientIP) {
		resp.SetRcode(req, dns.RcodeRefused)
		if dep.OnLimiterDrop != nil {
			dep.OnLimiterDrop("query_rate")
		}
		return resp
	}

	qname := primaryQname(req)
	if dep.ResponseLimiter != nil && !dep.ResponseLimiter.Allow(clientIP, qname) {
		resp.SetRcode(req, dns.RcodeRefused)
		if dep.OnLimiterDrop != nil {
			if st.DNSResponseLimitMode == "rrl" {
				dep.OnLimiterDrop("response_rrl")
			} else {
				dep.OnLimiterDrop("response_sliding")
			}
		}
		return resp
	}

	if dep.Resolver != nil {
		for _, q := range req.Question {
			dep.Resolver.HandleQuestion(ctx, q, resp)
		}
	}

	clampEDNS(resp, st.DNSMaxEDNSUDPPayload)

	if st.DNSAmplificationMaxRatio > 0 {
		clampAmplified(req, resp, st.DNSAmplificationMaxRatio)
	}

	if dep.ResponseLimiter != nil {
		dep.ResponseLimiter.RecordResponse(clientIP, qname)
	}

	return resp
}

func hasANYQuestion(req *dns.Msg) bool {
	for _, q := range req.Question {
		if q.Qtype == dns.TypeANY {
			return true
		}
	}
	return false
}

func primaryQname(req *dns.Msg) string {
	if len(req.Question) > 0 {
		return req.Question[0].Name
	}
	return "."
}

func clampAmplified(req, resp *dns.Msg, maxRatio int) {
	if maxRatio <= 0 {
		return
	}
	rw, e1 := req.Pack()
	pw, e2 := resp.Pack()
	if e1 != nil || e2 != nil || len(rw) == 0 {
		return
	}
	if len(pw) <= maxRatio*len(rw) {
		return
	}
	resp.Answer = nil
	resp.Ns = nil
	resp.Extra = nil
	resp.SetRcode(req, dns.RcodeServerFailure)
}

func clampEDNS(resp *dns.Msg, maxPayload uint16) {
	if maxPayload == 0 {
		return
	}
	opt := resp.IsEdns0()
	if opt == nil {
		return
	}
	if opt.UDPSize() > maxPayload {
		opt.SetUDPSize(maxPayload)
	}
}
