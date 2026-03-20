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
	"dnsplane/dnssecsign"
	"dnsplane/dnssecvalidate"
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
	FilterHealthyUpstreamEndpoints(eps []dnsservers.UpstreamEndpoint) []dnsservers.UpstreamEndpoint
	RecordUpstreamForwardSuccess(healthKey string)
	// TryFastLocalOrCache: single-lock local-then-cache for non-PTR; returns handled if answered from RAM.
	// isStale is true when the cache entry is expired but returned for stale-while-revalidate.
	TryFastLocalOrCache(qname, recordType string, qtypePTR bool) (handled bool, local []dns.RR, cache *dns.RR, isStale bool)
}

// UpstreamClient issues DNS queries to upstream resolvers.
type UpstreamClient interface {
	Query(ctx context.Context, question dns.Question, ep dnsservers.UpstreamEndpoint) (*dns.Msg, error)
}

// QueryLogger records human-friendly resolver activity.
type QueryLogger func(format string, args ...interface{})

// QueryObserver receives structured fields after each fast-path resolve (optional; for slog/metrics).
// outcome is one of: local, cache, upstream, none, blocked. upstream is address:port when outcome is upstream.
// recordSummary is a short text for the first answer (or "no answer", blocked reply, etc.).
type QueryObserver func(qname, qtype, outcome, upstream, recordSummary string, elapsed time.Duration)

// Config defines the resolver dependencies.
type Config struct {
	Store           Store
	Upstream        UpstreamClient
	Logger          QueryLogger
	ErrorLogger     ErrorLogger
	QueryObserver   QueryObserver
	UpstreamTimeout time.Duration
	// DNSSECSigner when non-nil signs local authoritative answers when the client sets DO (EDNS).
	DNSSECSigner *dnssecsign.Signer
}

// Resolver answers DNS questions using local records, cache, and upstream servers.
type Resolver struct {
	store           Store
	upstream        UpstreamClient
	logger          QueryLogger
	errorLogger     ErrorLogger
	queryObserver   QueryObserver
	upstreamTimeout time.Duration
	dnssecSigner    *dnssecsign.Signer
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
		queryObserver:   cfg.QueryObserver,
		upstreamTimeout: timeout,
		dnssecSigner:    cfg.DNSSECSigner,
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
	endpoint dnsservers.UpstreamEndpoint
	msg      *dns.Msg
	err      error
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
//
// A/AAAA: local/cache is consulted before adblock so a warm cache path does not pay GetBlockList+IsBlocked
// (two extra locks and work) on every query. If a name is on the blocklist but still has a positive cache
// entry, the cached answer is served until TTL expires.
func (r *Resolver) resolveFastPath(ctx context.Context, question dns.Question, response *dns.Msg) {
	t0 := time.Now()
	recordType := dns.TypeToString[question.Qtype]
	if recordType == "" {
		recordType = perfQTypeString(question)
	}
	qtypeKey := perfQTypeString(question)
	isPTR := question.Qtype == dns.TypePTR

	// Local/cache first without loading settings — one RLock (TryFastLocalOrCache) instead of
	// GetResolverSettings + TryFastLocalOrCache; matches the old dedicated A/cache hot path.
	if !isPTR {
		handled, loc, crr, isStale := r.store.TryFastLocalOrCache(question.Name, recordType, false)
		if handled {
			if len(loc) > 0 {
				r.processCachedRecords(ctx, question, loc, response)
				prep := uint64(time.Since(t0))
				data.RecordResolverAResolve(data.PerfOutcomeLocal, prep, prep, 0, 0, 0, qtypeKey)
				r.observeQuery(question, "local", "", rrOneLine(loc[0]), t0)
				return
			}
			if crr != nil {
				r.store.IncrementCacheHits()
				r.processCacheRecord(question, crr, response)
				prep := uint64(time.Since(t0))
				data.RecordResolverAResolve(data.PerfOutcomeCache, prep, prep, 0, 0, 0, qtypeKey)
				r.observeQuery(question, "cache", "", rrOneLine(*crr), t0)
				if isStale {
					go r.backgroundRefresh(question)
				}
				return
			}
		}
	}

	if question.Qtype == dns.TypeA || question.Qtype == dns.TypeAAAA {
		if r.checkBlocked(question) {
			r.processBlockedDomain(question, response)
			r.observeQuery(question, "blocked", "", msgAnswerSummary(response), t0)
			return
		}
	}

	settings := r.store.GetResolverSettings()
	allServers := r.store.GetServers()
	dnsServers := dnsservers.GetUpstreamEndpointsForQuery(allServers, question.Name, true)
	useWhitelist := false
	for _, s := range allServers {
		if s.Active && dnsservers.ServerMatchesQuery(s, question.Name) {
			useWhitelist = true
			break
		}
	}
	var fallbackEp dnsservers.UpstreamEndpoint
	if !useWhitelist && settings.FallbackServerIP != "" && settings.FallbackServerPort != "" {
		fallbackEp = dnsservers.FallbackEndpoint(settings.FallbackServerIP, settings.FallbackServerPort, settings.FallbackServerTransport)
	}
	serversToQuery := dnsServers
	if fallbackEp.Addr != "" {
		already := false
		for _, s := range dnsServers {
			if s.HealthKey() == fallbackEp.HealthKey() {
				already = true
				break
			}
		}
		if !already {
			serversToQuery = append(append([]dnsservers.UpstreamEndpoint(nil), dnsServers...), fallbackEp)
		}
	}
	serversToQuery = r.store.FilterHealthyUpstreamEndpoints(serversToQuery)
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
				r.log("Query: %s, Error querying DNS server (%s): %v\n", question.Name, srv.String(), err)
			}
			up := &upstreamResult{endpoint: srv, msg: resp, err: err}
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
				r.processCachedRecords(ctx, question, pr.local, response)
				prep := localNs
				if cacheDone && cacheNs > prep {
					prep = cacheNs
				}
				recordPerf(data.PerfOutcomeLocal, uint64(time.Since(t0)), prep, 0, 0)
				r.observeQuery(question, "local", "", rrOneLine(pr.local[0]), t0)
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
			r.observeQuery(question, "cache", "", rrOneLine(*cacheHit), t0)
			return
		}
		if localDone && cacheDone && cacheHit == nil && firstUp != nil {
			cancel()
			r.store.RecordUpstreamForwardSuccess(firstUp.endpoint.HealthKey())
			r.processUpstreamAnswer(ctx, question, firstUp.msg, response)
			p := prepNs()
			var w uint64
			if maxUpNs > p {
				w = maxUpNs - p
			}
			recordPerf(data.PerfOutcomeUpstream, uint64(time.Since(t0)), p, maxUpNs, w)
			r.observeQuery(question, "upstream", firstUp.endpoint.HealthKey(), firstAnswerSummary(firstUp.msg), t0)
			return
		}
		if localDone && cacheDone && cacheHit == nil && upstreamSeen == upstreamTotal && firstUp == nil {
			cancel()
			r.log("Query: %s, No response\n", question.Name)
			recordPerf(data.PerfOutcomeNone, uint64(time.Since(t0)), prepNs(), maxUpNs, 0)
			r.observeQuery(question, "none", "", "no answer", t0)
			return
		}
	}
}

func (r *Resolver) handlePTRQuestion(ctx context.Context, question dns.Question, response *dns.Msg) {
	t0 := time.Now()
	settings := r.store.GetResolverSettings()
	ptrRecords := r.store.LookupLocalRRs(question.Name, "PTR", settings.DNSRecordSettings.AutoBuildPTRFromA)
	if len(ptrRecords) > 0 {
		r.processCachedRecords(ctx, question, ptrRecords, response)
		r.observeQuery(question, "local", "", rrOneLine(ptrRecords[0]), t0)
		return
	}
	r.log("PTR record not found in dnsrecords.json\n")
	r.resolveFastPath(ctx, question, response)
}

// processUpstreamAnswer appends the upstream answer to the response and caches it.
// The response's Authoritative and Rcode are set from the upstream message.
func (r *Resolver) processUpstreamAnswer(ctx context.Context, question dns.Question, answer *dns.Msg, response *dns.Msg) {
	req := RequestFromContext(ctx)
	settings := r.store.GetResolverSettings()
	outcome, servfail := dnssecvalidate.ApplyToUpstreamAnswer(req, answer, question, settings)
	data.RecordDNSSECOutcome(outcome)
	if servfail {
		if req != nil {
			response.SetRcode(req, dns.RcodeServerFailure)
		} else {
			response.Rcode = dns.RcodeServerFailure
		}
		return
	}
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = answer.Authoritative
	response.Rcode = answer.Rcode
	response.AuthenticatedData = answer.AuthenticatedData
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

func (r *Resolver) processCachedRecords(ctx context.Context, question dns.Question, cachedRecords []dns.RR, response *dns.Msg) {
	if len(cachedRecords) == 0 {
		return
	}
	response.Answer = append(response.Answer, cachedRecords...)
	if r.dnssecSigner != nil {
		req := RequestFromContext(ctx)
		r.dnssecSigner.SignLocalAnswerIfDO(req, question, cachedRecords, response)
	}
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
	minTTL := uint32(settings.MinCacheTTLSeconds)
	if settings.MinCacheTTLSeconds == 0 {
		minTTL = 600
	}
	cache := store.GetCacheRecords()
	for i := range rrs {
		rr := rrs[i]
		cache = dnsrecordcache.Add(cache, &rr, minTTL)
	}
	store.UpdateCacheRecords(cache)
}

// backgroundRefresh queries upstream for a stale cache entry and updates the cache.
func (r *Resolver) backgroundRefresh(question dns.Question) {
	if r == nil || r.store == nil || r.upstream == nil {
		return
	}
	settings := r.store.GetResolverSettings()
	allServers := r.store.GetServers()
	servers := dnsservers.GetUpstreamEndpointsForQuery(allServers, question.Name, true)
	if len(servers) == 0 {
		if settings.FallbackServerIP != "" && settings.FallbackServerPort != "" {
			servers = []dnsservers.UpstreamEndpoint{
				dnsservers.FallbackEndpoint(settings.FallbackServerIP, settings.FallbackServerPort, settings.FallbackServerTransport),
			}
		}
	}
	servers = r.store.FilterHealthyUpstreamEndpoints(servers)
	if len(servers) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.upstreamTimeout)
	defer cancel()
	for _, srv := range servers {
		resp, err := r.upstream.Query(ctx, question, srv)
		if err == nil && resp != nil && resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 0 {
			cacheDNSResponse(r.store, resp.Answer)
			return
		}
	}
}

func (r *Resolver) log(format string, args ...interface{}) {
	if r == nil || r.logger == nil {
		return
	}
	r.logger(format, args...)
}

func (r *Resolver) observeQuery(question dns.Question, outcome, upstream, recordSummary string, t0 time.Time) {
	if r == nil || r.queryObserver == nil {
		return
	}
	r.queryObserver(question.Name, perfQTypeString(question), outcome, upstream, recordSummary, time.Since(t0))
}

func rrOneLine(rr dns.RR) string {
	s := rr.String()
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}

func firstAnswerSummary(msg *dns.Msg) string {
	if msg == nil || len(msg.Answer) == 0 {
		return "no answer"
	}
	return rrOneLine(msg.Answer[0])
}

func msgAnswerSummary(msg *dns.Msg) string {
	if msg == nil {
		return "—"
	}
	if len(msg.Answer) > 0 {
		return rrOneLine(msg.Answer[0])
	}
	if msg.Rcode != dns.RcodeSuccess {
		return dns.RcodeToString[msg.Rcode] + " (blocked)"
	}
	return "no answer"
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
