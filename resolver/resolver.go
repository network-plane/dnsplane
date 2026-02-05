package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"

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
	UpdateCacheRecords([]dnsrecordcache.CacheRecord)
	GetServers() []dnsservers.DNSServer
	GetBlockList() *adblock.BlockList
	IncrementCacheHits()
	IncrementQueriesAnswered()
	IncrementTotalBlocks()
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
	case dns.TypeA:
		r.handleAQuestion(ctx, question, response)
	default:
		r.handleDNSServers(ctx, question, response)
	}
	r.store.IncrementQueriesAnswered()
}

func (r *Resolver) handleAQuestion(ctx context.Context, question dns.Question, response *dns.Msg) {
	// Check if domain is blocked by adblock
	if r.checkBlocked(question) {
		r.processBlockedDomain(question, response)
		return
	}

	settings := r.store.GetResolverSettings()
	records := r.store.GetRecords()
	recordType := dns.TypeToString[question.Qtype]
	cachedRecords := dnsrecords.FindAllRecords(records, question.Name, recordType, settings.DNSRecordSettings.AutoBuildPTRFromA)
	if len(cachedRecords) > 0 {
		r.processCachedRecords(question, cachedRecords, response)
		return
	}

	cache := r.store.GetCacheRecords()
	if cached := findCacheRecord(cache, question.Name, recordType, r.errorLogger); cached != nil {
		r.store.IncrementCacheHits()
		r.processCacheRecord(question, cached, response)
		return
	}

	r.handleDNSServers(ctx, question, response)
}

func (r *Resolver) handlePTRQuestion(ctx context.Context, question dns.Question, response *dns.Msg) {
	settings := r.store.GetResolverSettings()
	records := r.store.GetRecords()
	recordType := dns.TypeToString[question.Qtype]

	ptrRecords := dnsrecords.FindAllRecords(records, question.Name, recordType, settings.DNSRecordSettings.AutoBuildPTRFromA)
	if len(ptrRecords) > 0 {
		r.processCachedRecords(question, ptrRecords, response)
		return
	}

	r.log("PTR record not found in dnsrecords.json\n")
	r.handleDNSServers(ctx, question, response)
}

func (r *Resolver) handleDNSServers(ctx context.Context, question dns.Question, response *dns.Msg) {
	// Check if domain is blocked by adblock before querying upstream
	if r.checkBlocked(question) {
		r.processBlockedDomain(question, response)
		return
	}

	settings := r.store.GetResolverSettings()
	dnsServers := dnsservers.GetDNSArray(r.store.GetServers(), true)
	fallback := ""
	if settings.FallbackServerIP != "" && settings.FallbackServerPort != "" {
		fallback = fmt.Sprintf("%s:%s", settings.FallbackServerIP, settings.FallbackServerPort)
	}

	ctx, cancel := context.WithTimeout(ctx, r.upstreamTimeout)
	defer cancel()

	found := false
	for res := range r.queryAllDNSServers(ctx, question, dnsServers) {
		if res.err != nil {
			r.log("Query: %s, Error querying DNS server (%s): %v\n", question.Name, res.server, res.err)
			continue
		}
		if res.msg == nil {
			continue
		}
		if len(res.msg.Answer) == 0 {
			continue
		}
		if res.msg.MsgHdr.Authoritative {
			r.processAuthoritativeAnswer(question, res.msg, response)
			found = true
			cancel()
			break
		}
	}

	if !found {
		r.handleFallbackServer(ctx, question, fallback, response)
	}
}

type upstreamResult struct {
	server string
	msg    *dns.Msg
	err    error
}

func (r *Resolver) queryAllDNSServers(ctx context.Context, question dns.Question, servers []string) <-chan upstreamResult {
	if len(servers) == 0 || r.upstream == nil {
		results := make(chan upstreamResult)
		close(results)
		return results
	}
	// Buffered channel so workers can send without blocking when the consumer
	// breaks out early (e.g. after first authoritative answer). Prevents
	// goroutine leak that would otherwise exhaust resources over time.
	results := make(chan upstreamResult, len(servers))

	g, ctx := errgroup.WithContext(ctx)
	for _, server := range servers {
		server := server
		g.Go(func() error {
			resp, err := r.upstream.Query(ctx, question, server)
			select {
			case results <- upstreamResult{server: server, msg: resp, err: err}:
			case <-ctx.Done():
			}
			return nil
		})
	}

	go func() {
		_ = g.Wait()
		close(results)
	}()

	return results
}

func (r *Resolver) handleFallbackServer(ctx context.Context, question dns.Question, fallbackServer string, response *dns.Msg) {
	if fallbackServer == "" || r.upstream == nil {
		r.log("Query: %s, No response\n", question.Name)
		return
	}

	resp, err := r.upstream.Query(ctx, question, fallbackServer)
	if err != nil {
		r.log("Query: %s, Error querying fallback DNS server (%s): %v\n", question.Name, fallbackServer, err)
		return
	}
	if resp == nil || len(resp.Answer) == 0 {
		r.log("Query: %s, No response\n", question.Name)
		return
	}

	response.Answer = append(response.Answer, resp.Answer...)
	r.log("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, resp.Answer[0].String(), fallbackServer)
	cacheDNSResponse(r.store, resp.Answer)
}

func (r *Resolver) processAuthoritativeAnswer(question dns.Question, answer *dns.Msg, response *dns.Msg) {
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = true
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

func (r *Resolver) processCachedRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	r.processCachedRecords(question, []dns.RR{*cachedRecord}, response)
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

func findCacheRecord(cacheRecords []dnsrecordcache.CacheRecord, name string, recordType string, errorLog ErrorLogger) *dns.RR {
	now := time.Now()
	for _, record := range cacheRecords {
		if dnsrecords.NormalizeRecordNameKey(record.DNSRecord.Name) == dnsrecords.NormalizeRecordNameKey(name) &&
			dnsrecords.NormalizeRecordType(record.DNSRecord.Type) == dnsrecords.NormalizeRecordType(recordType) {
			if now.Before(record.Expiry) {
				remainingTTL := uint32(record.Expiry.Sub(now).Seconds())
				return dnsRecordToRR(&record.DNSRecord, remainingTTL, errorLog)
			}
		}
	}
	return nil
}

func dnsRecordToRR(dnsRecord *dnsrecords.DNSRecord, ttl uint32, errorLog ErrorLogger) *dns.RR {
	recordString := fmt.Sprintf("%s %d IN %s %s", dnsRecord.Name, ttl, dnsRecord.Type, dnsRecord.Value)
	rr, err := dns.NewRR(recordString)
	if err != nil {
		if errorLog != nil {
			errorLog("resolver: error converting DNSRecord to RR", "error", err)
		}
		return nil
	}
	return &rr
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
