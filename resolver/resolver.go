// Package resolver implements the DNS resolver.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"dnsplane/adblock"
	"dnsplane/data"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
	"dnsplane/dnsservers"
)

// ErrorLogger logs errors (e.g. conversion failures). Optional.
type ErrorLogger func(msg string, keyValues ...any)

// Store abstracts access to resolver data.
type Store interface {
	GetResolverSettings() data.DNSResolverSettings
	GetRecords() []dnsrecords.DNSRecord
	GetCacheRecords() []dnsrecordcache.CacheRecord
	LookupLocalRRs(name, recordType string, autoBuildPTRFromA bool) []dns.RR
	LookupCacheRR(name, recordType string) *dns.RR
	UpdateCacheRecords([]dnsrecordcache.CacheRecord)
	GetServers() []dnsservers.DNSServer
	GetBlockList() *adblock.BlockList
	IncrementCacheHits()
	IncrementQueriesAnswered()
	IncrementTotalBlocks()
	// HasAnyLocalRecords / HasAnyCachedRecords enable short-circuits without copying slices.
	HasAnyLocalRecords() bool
	HasAnyCachedRecords() bool
	FilterHealthyUpstreamAddresses(addrs []string) []string
	RecordUpstreamForwardSuccess(addressPort string)
	// TryFastLocalOrCache: single-lock local-then-cache for non-PTR; returns handled if answered from RAM.
	TryFastLocalOrCache(qname, recordType string, qtypePTR bool) (handled bool, local []dns.RR, cache *dns.RR)
}

// UpstreamClient issues DNS queries to upstream resolvers.
type UpstreamClient interface {
	Query(ctx context.Context, question dns.Question, server string) (*dns.Msg, error)
}

// QueryLogger records human-friendly resolver activity.
type QueryLogger func(format string, args ...interface{})

// Config defines the resolver dependencies.
type Config struct {
	Store           Store
	Upstream        UpstreamClient
	Logger          QueryLogger
	ErrorLogger     ErrorLogger
	UpstreamTimeout time.Duration
}

// Resolver answers DNS questions using local records, cache, and upstream servers.
type Resolver struct {
	store           Store
	upstream        UpstreamClient
	logger          QueryLogger
	errorLogger     ErrorLogger
	upstreamTimeout time.Duration
}

// New constructs a Resolver using the provided configuration.
func New(cfg Config) *Resolver {
	timeout := cfg.UpstreamTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Resolver{
		store:           cfg.Store,
		upstream:        cfg.Upstream,
		logger:          cfg.Logger,
		errorLogger:     cfg.ErrorLogger,
		upstreamTimeout: timeout,
	}
}

// HandleQuestion resolves a single DNS question and appends answers to response.
func (r *Resolver) HandleQuestion(ctx context.Context, question dns.Question, response *dns.Msg) {
	if r == nil || r.store == nil {
		return
	}

	switch question.Qtype {
	case dns.TypePTR:
		r.handlePTRQuestion(ctx, question, response)
	default:
		r.resolveFastPath(ctx, question, response)
	}
	r.store.IncrementQueriesAnswered()
}

type upstreamResult struct {
	server string
	msg    *dns.Msg
	err    error
}

type parKind int

const (
	parKindLocal parKind = iota
	parKindCache
	parKindUpstream
)

type parMsg struct {
	kind    parKind
	local   []dns.RR
	cache   *dns.RR
	up      *upstreamResult
	elapsed time.Duration // local/cache: scan time; upstream: since query start
}

func perfQTypeString(q dns.Question) string {
	if s := dns.TypeToString[q.Qtype]; s != "" {
		return s
	}
	return "T" + strconv.Itoa(int(q.Qtype))
}

// resolveFastPath runs local, cache, and all upstreams at once for any QTYPE. Priority: local > cache > first upstream success.
func (r *Resolver) resolveFastPath(ctx context.Context, question dns.Question, response *dns.Msg) {
	if question.Qtype == dns.TypeA || question.Qtype == dns.TypeAAAA {
		if r.checkBlocked(question) {
			r.processBlockedDomain(question, response)
			return
		}
	}
	t0 := time.Now()
	settings := r.store.GetResolverSettings()
	recordType := dns.TypeToString[question.Qtype]
	if recordType == "" {
		recordType = perfQTypeString(question)
	}
	qtypeKey := perfQTypeString(question)

	isPTR := question.Qtype == dns.TypePTR
	if !isPTR {
		handled, loc, crr := r.store.TryFastLocalOrCache(question.Name, recordType, false)
		if handled {
			if len(loc) > 0 {
				r.processCachedRecords(question, loc, response)
				prep := uint64(time.Since(t0))
				data.RecordResolverAResolve(data.PerfOutcomeLocal, prep, prep, 0, 0, 0, qtypeKey)
				return
			}
			if crr != nil {
				r.store.IncrementCacheHits()
				r.processCacheRecord(question, crr, response)
				prep := uint64(time.Since(t0))
				data.RecordResolverAResolve(data.PerfOutcomeCache, prep, prep, 0, 0, 0, qtypeKey)
				return
			}
		}
	}

	allServers := r.store.GetServers()
	dnsServers := dnsservers.GetServersForQuery(allServers, question.Name, true)
	useWhitelist := false
	for _, s := range allServers {
		if s.Active && dnsservers.ServerMatchesQuery(s, question.Name) {
			useWhitelist = true
			break
		}
	}
	fallback := ""
	if !useWhitelist && settings.FallbackServerIP != "" && settings.FallbackServerPort != "" {
		fallback = fmt.Sprintf("%s:%s", settings.FallbackServerIP, settings.FallbackServerPort)
	}
	serversToQuery := dnsServers
	if fallback != "" {
		already := false
		for _, s := range dnsServers {
			if s == fallback {
				already = true
				break
			}
		}
		if !already {
			serversToQuery = append(append([]string(nil), dnsServers...), fallback)
		}
	}
	serversToQuery = r.store.FilterHealthyUpstreamAddresses(serversToQuery)
	nUp := len(serversToQuery)

	ctx, cancel := context.WithTimeout(ctx, r.upstreamTimeout)
	defer cancel()

	capCh := 2 + nUp
	ch := make(chan parMsg, capCh)

	if isPTR {
		go func() {
			t1 := time.Now()
			lr := r.store.LookupLocalRRs(question.Name, recordType, settings.DNSRecordSettings.AutoBuildPTRFromA)
			ch <- parMsg{kind: parKindLocal, local: lr, elapsed: time.Since(t1)}
		}()
		if !settings.CacheRecords {
			ch <- parMsg{kind: parKindCache, elapsed: 0}
		} else if r.store.HasAnyCachedRecords() {
			go func() {
				t1 := time.Now()
				cr := r.store.LookupCacheRR(question.Name, recordType)
				ch <- parMsg{kind: parKindCache, cache: cr, elapsed: time.Since(t1)}
			}()
		} else {
			ch <- parMsg{kind: parKindCache, elapsed: 0}
		}
	} else {
		// Local and cache already ruled out under one lock; only upstream matters.
		ch <- parMsg{kind: parKindLocal, elapsed: 0}
		ch <- parMsg{kind: parKindCache, elapsed: 0}
	}
	for _, srv := range serversToQuery {
		srv := srv
		go func() {
			resp, err := r.upstream.Query(ctx, question, srv)
			if err != nil && r != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
				r.log("Query: %s, Error querying DNS server (%s): %v\n", question.Name, srv, err)
			}
			up := &upstreamResult{server: srv, msg: resp, err: err}
			select {
			case ch <- parMsg{kind: parKindUpstream, up: up, elapsed: time.Since(t0)}:
			case <-ctx.Done():
			}
		}()
	}

	var localDone, cacheDone bool
	var localNs, cacheNs uint64
	var cacheHit *dns.RR
	var firstUp *upstreamResult
	var maxUpNs uint64
	var upMu sync.Mutex
	upstreamSeen := 0
	upstreamTotal := nUp

	recordPerf := func(outcome int, totalNs, prepNs, maxUpstreamNs, waitNs uint64) {
		data.RecordResolverAResolve(outcome, totalNs, prepNs, maxUpstreamNs, waitNs, nUp, qtypeKey)
	}

	for i := 0; i < capCh; i++ {
		pr := <-ch
		switch pr.kind {
		case parKindLocal:
			localDone = true
			localNs = uint64(pr.elapsed)
			if len(pr.local) > 0 {
				cancel()
				r.processCachedRecords(question, pr.local, response)
				prep := localNs
				if cacheDone && cacheNs > prep {
					prep = cacheNs
				}
				recordPerf(data.PerfOutcomeLocal, uint64(time.Since(t0)), prep, 0, 0)
				return
			}
		case parKindCache:
			cacheDone = true
			cacheNs = uint64(pr.elapsed)
			if pr.cache != nil {
				cacheHit = pr.cache
			}
		case parKindUpstream:
			upstreamSeen++
			if uint64(pr.elapsed) > maxUpNs {
				maxUpNs = uint64(pr.elapsed)
			}
			if pr.up != nil && pr.up.err == nil && pr.up.msg != nil &&
				pr.up.msg.Rcode == dns.RcodeSuccess && len(pr.up.msg.Answer) > 0 {
				upMu.Lock()
				if firstUp == nil {
					firstUp = pr.up
				}
				upMu.Unlock()
			}
		}

		prepNs := func() uint64 {
			p := localNs
			if cacheNs > p {
				p = cacheNs
			}
			return p
		}
		if localDone && cacheDone && cacheHit != nil {
			cancel()
			r.store.IncrementCacheHits()
			r.processCacheRecord(question, cacheHit, response)
			p := prepNs()
			recordPerf(data.PerfOutcomeCache, uint64(time.Since(t0)), p, maxUpNs, 0)
			return
		}
		if localDone && cacheDone && cacheHit == nil && firstUp != nil {
			cancel()
			r.store.RecordUpstreamForwardSuccess(firstUp.server)
			r.processUpstreamAnswer(question, firstUp.msg, response)
			p := prepNs()
			var w uint64
			if maxUpNs > p {
				w = maxUpNs - p
			}
			recordPerf(data.PerfOutcomeUpstream, uint64(time.Since(t0)), p, maxUpNs, w)
			return
		}
		if localDone && cacheDone && cacheHit == nil && upstreamSeen == upstreamTotal && firstUp == nil {
			cancel()
			r.log("Query: %s, No response\n", question.Name)
			recordPerf(data.PerfOutcomeNone, uint64(time.Since(t0)), prepNs(), maxUpNs, 0)
			return
		}
	}
}

func (r *Resolver) handlePTRQuestion(ctx context.Context, question dns.Question, response *dns.Msg) {
	settings := r.store.GetResolverSettings()
	ptrRecords := r.store.LookupLocalRRs(question.Name, "PTR", settings.DNSRecordSettings.AutoBuildPTRFromA)
	if len(ptrRecords) > 0 {
		r.processCachedRecords(question, ptrRecords, response)
		return
	}
	r.log("PTR record not found in dnsrecords.json\n")
	r.resolveFastPath(ctx, question, response)
}

// processUpstreamAnswer appends the upstream answer to the response and caches it.
// The response's Authoritative and Rcode are set from the upstream message.
func (r *Resolver) processUpstreamAnswer(question dns.Question, answer *dns.Msg, response *dns.Msg) {
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = answer.Authoritative
	response.Rcode = answer.Rcode
	if len(answer.Answer) > 0 {
		record := answer.Answer[0]
		name := record.Header().Name
		if len(name) > 0 {
			name = name[:len(name)-1]
		}
		r.log("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, record.String(), name)
	}
	cacheDNSResponse(r.store, answer.Answer)
}

func (r *Resolver) processCachedRecords(question dns.Question, cachedRecords []dns.RR, response *dns.Msg) {
	if len(cachedRecords) == 0 {
		return
	}
	response.Answer = append(response.Answer, cachedRecords...)
	response.Authoritative = true
	if len(cachedRecords) > 0 {
		r.log("Query: %s, Reply: %d record(s), Method: dnsrecords.json\n", question.Name, len(cachedRecords))
		for _, rr := range cachedRecords {
			r.log("  %s\n", rr.String())
		}
	}
	cacheDNSResponse(r.store, cachedRecords)
}

func (r *Resolver) processCacheRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	r.log("Query: %s, Reply: %s, Method: dnscache.json\n", question.Name, (*cachedRecord).String())
}

func cacheDNSResponse(store Store, rrs []dns.RR) {
	if store == nil || len(rrs) == 0 {
		return
	}
	settings := store.GetResolverSettings()
	if !settings.CacheRecords {
		return
	}
	cache := store.GetCacheRecords()
	for i := range rrs {
		rr := rrs[i]
		cache = dnsrecordcache.Add(cache, &rr)
	}
	store.UpdateCacheRecords(cache)
}

func (r *Resolver) log(format string, args ...interface{}) {
	if r == nil || r.logger == nil {
		return
	}
	r.logger(format, args...)
}

// checkBlocked checks if a domain is blocked by the adblock list.
func (r *Resolver) checkBlocked(question dns.Question) bool {
	if r == nil || r.store == nil {
		return false
	}
	blockList := r.store.GetBlockList()
	if blockList == nil {
		return false
	}

	// Normalize domain name (remove trailing dot)
	domain := strings.TrimSuffix(question.Name, ".")
	return blockList.IsBlocked(domain)
}

// processBlockedDomain returns a blocked response (0.0.0.0 for A, :: for AAAA).
func (r *Resolver) processBlockedDomain(question dns.Question, response *dns.Msg) {
	if r == nil || r.store == nil {
		return
	}

	r.store.IncrementTotalBlocks()
	response.Authoritative = true

	var blockedIP string
	var recordType uint16

	switch question.Qtype {
	case dns.TypeA:
		blockedIP = "0.0.0.0"
		recordType = dns.TypeA
	case dns.TypeAAAA:
		blockedIP = "::"
		recordType = dns.TypeAAAA
	default:
		// For other types, return NXDOMAIN or empty response
		response.Rcode = dns.RcodeNameError
		r.log("Query: %s, Blocked (adblock), Type: %s\n", question.Name, dns.TypeToString[question.Qtype])
		return
	}

	// Create a blocked response record
	recordString := fmt.Sprintf("%s 300 IN %s %s", question.Name, dns.TypeToString[recordType], blockedIP)
	rr, err := dns.NewRR(recordString)
	if err != nil {
		r.log("Query: %s, Error creating blocked response: %v\n", question.Name, err)
		return
	}

	response.Answer = append(response.Answer, rr)
	r.log("Query: %s, Blocked (adblock), Reply: %s\n", question.Name, rr.String())
}
