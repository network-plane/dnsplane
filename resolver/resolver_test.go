// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"

	"dnsplane/adblock"
	"dnsplane/config"
	"dnsplane/data"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
	"dnsplane/dnsservers"
)

// recordingUpstream records each (question name, server) Query call and returns a minimal success response.
type recordingUpstream struct {
	mu      sync.Mutex
	queries []struct{ name, server string }
}

func (u *recordingUpstream) Query(ctx context.Context, question dns.Question, server string) (*dns.Msg, error) {
	u.mu.Lock()
	u.queries = append(u.queries, struct{ name, server string }{question.Name, server})
	u.mu.Unlock()
	msg := &dns.Msg{}
	msg.SetReply(&dns.Msg{Question: []dns.Question{question}})
	msg.Rcode = dns.RcodeSuccess
	hdr := func() dns.RR_Header {
		return dns.RR_Header{Name: question.Name, Rrtype: question.Qtype, Class: dns.ClassINET, Ttl: 60}
	}
	var ans dns.RR
	switch question.Qtype {
	case dns.TypeAAAA:
		ans = &dns.AAAA{Hdr: hdr(), AAAA: net.IPv6loopback}
	case dns.TypeMX:
		ans = &dns.MX{Hdr: hdr(), Preference: 10, Mx: "mail.example.com."}
	case dns.TypePTR:
		ans = &dns.PTR{Hdr: hdr(), Ptr: "resolved.example.com."}
	default:
		ans = &dns.A{Hdr: hdr(), A: []byte{1, 2, 3, 4}}
	}
	msg.Answer = []dns.RR{ans}
	return msg, nil
}

func (u *recordingUpstream) recorded() []struct{ name, server string } {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := make([]struct{ name, server string }, len(u.queries))
	copy(out, u.queries)
	return out
}

func (u *recordingUpstream) reset() {
	u.mu.Lock()
	u.queries = nil
	u.mu.Unlock()
}

// whitelistIntegrationStore implements Store with one global and one whitelist server.
type whitelistIntegrationStore struct {
	servers []dnsservers.DNSServer
	config  config.Config
}

func (s *whitelistIntegrationStore) GetResolverSettings() data.DNSResolverSettings { return s.config }
func (s *whitelistIntegrationStore) GetRecords() []dnsrecords.DNSRecord            { return nil }
func (s *whitelistIntegrationStore) GetCacheRecords() []dnsrecordcache.CacheRecord { return nil }
func (s *whitelistIntegrationStore) LookupLocalRRs(name, recordType string, autoBuildPTR bool) []dns.RR {
	return dnsrecords.FindAllRecords(s.GetRecords(), name, recordType, autoBuildPTR)
}
func (s *whitelistIntegrationStore) LookupCacheRR(string, string) *dns.RR              { return nil }
func (s *whitelistIntegrationStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord) {}
func (s *whitelistIntegrationStore) GetServers() []dnsservers.DNSServer                { return s.servers }
func (s *whitelistIntegrationStore) GetBlockList() *adblock.BlockList {
	return adblock.NewBlockList()
}
func (s *whitelistIntegrationStore) IncrementCacheHits()                                {}
func (s *whitelistIntegrationStore) IncrementQueriesAnswered()                          {}
func (s *whitelistIntegrationStore) IncrementTotalBlocks()                              {}
func (s *whitelistIntegrationStore) HasAnyLocalRecords() bool                           { return len(s.GetRecords()) > 0 }
func (s *whitelistIntegrationStore) HasAnyCachedRecords() bool                          { return len(s.GetCacheRecords()) > 0 }
func (s *whitelistIntegrationStore) FilterHealthyUpstreamAddresses(a []string) []string { return a }
func (s *whitelistIntegrationStore) RecordUpstreamForwardSuccess(string)                {}
func (s *whitelistIntegrationStore) TryFastLocalOrCache(string, string, bool) (bool, []dns.RR, *dns.RR, bool) {
	return false, nil, nil, false
}

// TestWhitelistIntegration verifies that for a query matching a whitelist server only that server
// is used, and for a query that does not match only the global server is used (integration: resolver + GetServersForQuery).
func TestWhitelistIntegration(t *testing.T) {
	globalServer := dnsservers.DNSServer{
		Address: "8.8.8.8",
		Port:    "53",
		Active:  true,
	}
	whitelistServer := dnsservers.DNSServer{
		Address:         "192.168.5.5",
		Port:            "53",
		Active:          true,
		DomainWhitelist: []string{"internal.vodafoneinnovus.com"},
	}
	store := &whitelistIntegrationStore{
		servers: []dnsservers.DNSServer{globalServer, whitelistServer},
		config:  config.Config{}, // no fallback
	}
	rec := &recordingUpstream{}
	r := New(Config{
		Store:           store,
		Upstream:        rec,
		UpstreamTimeout: 2 * time.Second,
	})
	ctx := context.Background()

	// Query whitelisted domain: only whitelist server should be queried
	rec.reset()
	qWhitelist := dns.Question{Name: "api.internal.vodafoneinnovus.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	msgWhitelist := &dns.Msg{}
	msgWhitelist.SetQuestion(qWhitelist.Name, qWhitelist.Qtype)
	r.HandleQuestion(ctx, qWhitelist, msgWhitelist)
	got := rec.recorded()
	if len(got) == 0 {
		t.Fatal("expected at least one upstream query for whitelisted domain, got none")
	}
	for _, q := range got {
		if q.server != "192.168.5.5:53" {
			t.Errorf("whitelisted domain: expected only 192.168.5.5:53 to be queried, got server %q", q.server)
		}
	}

	// Query non-whitelisted domain: only global server should be queried
	rec.reset()
	qGlobal := dns.Question{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	msgGlobal := &dns.Msg{}
	msgGlobal.SetQuestion(qGlobal.Name, qGlobal.Qtype)
	r.HandleQuestion(ctx, qGlobal, msgGlobal)
	got = rec.recorded()
	if len(got) == 0 {
		t.Fatal("expected at least one upstream query for global domain, got none")
	}
	for _, q := range got {
		if q.server != "8.8.8.8:53" {
			t.Errorf("global domain: expected only 8.8.8.8:53 to be queried, got server %q", q.server)
		}
	}
}

// localRecordStore implements Store with a single local A record (no cache, no upstream used when local wins).
type localRecordStore struct {
	records []dnsrecords.DNSRecord
	config  config.Config
}

func (s *localRecordStore) GetResolverSettings() data.DNSResolverSettings { return s.config }
func (s *localRecordStore) GetRecords() []dnsrecords.DNSRecord            { return s.records }
func (s *localRecordStore) GetCacheRecords() []dnsrecordcache.CacheRecord { return nil }
func (s *localRecordStore) LookupLocalRRs(name, recordType string, autoBuildPTR bool) []dns.RR {
	return dnsrecords.FindAllRecords(s.records, name, recordType, autoBuildPTR)
}
func (s *localRecordStore) LookupCacheRR(string, string) *dns.RR               { return nil }
func (s *localRecordStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord)  {}
func (s *localRecordStore) GetServers() []dnsservers.DNSServer                 { return nil }
func (s *localRecordStore) GetBlockList() *adblock.BlockList                   { return adblock.NewBlockList() }
func (s *localRecordStore) IncrementCacheHits()                                {}
func (s *localRecordStore) IncrementQueriesAnswered()                          {}
func (s *localRecordStore) IncrementTotalBlocks()                              {}
func (s *localRecordStore) HasAnyLocalRecords() bool                           { return len(s.records) > 0 }
func (s *localRecordStore) HasAnyCachedRecords() bool                          { return false }
func (s *localRecordStore) FilterHealthyUpstreamAddresses(a []string) []string { return a }
func (s *localRecordStore) RecordUpstreamForwardSuccess(string)                {}
func (s *localRecordStore) TryFastLocalOrCache(qname, rt string, ptr bool) (bool, []dns.RR, *dns.RR, bool) {
	if ptr {
		return false, nil, nil, false
	}
	loc := dnsrecords.FindAllRecords(s.records, qname, rt, false)
	if len(loc) > 0 {
		return true, loc, nil, false
	}
	return false, nil, nil, false
}

// TestResolver_LocalRecordReturnsA is a minimal integration test: resolver with in-memory store
// holding one A record; one A query returns that record (local wins, no upstream).
func TestResolver_LocalRecordReturnsA(t *testing.T) {
	store := &localRecordStore{
		records: []dnsrecords.DNSRecord{
			{Name: "test.example.com.", Type: "A", Value: "1.2.3.4", TTL: 60},
		},
		config: config.Config{},
	}
	r := New(Config{
		Store:           store,
		Upstream:        &recordingUpstream{}, // not called when local hits
		UpstreamTimeout: 2 * time.Second,
	})
	ctx := context.Background()

	question := dns.Question{Name: "test.example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	response := &dns.Msg{}
	response.SetQuestion(question.Name, question.Qtype)
	r.HandleQuestion(ctx, question, response)

	if len(response.Answer) == 0 {
		t.Fatal("expected one A record in response, got none")
	}
	if response.Rcode != dns.RcodeSuccess {
		t.Errorf("response Rcode = %s, want Success", dns.RcodeToString[response.Rcode])
	}
	a, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Answer[0] type = %T, want *dns.A", response.Answer[0])
	}
	if a.A.String() != "1.2.3.4" {
		t.Errorf("A record value = %s, want 1.2.3.4", a.A.String())
	}
}

// emptyStore has no local records, no cache, no upstream servers, no fallback.
// Used to test resolution when nothing can answer (expect empty or NXDOMAIN-like response).
type emptyStore struct {
	config config.Config
}

func (s *emptyStore) GetResolverSettings() data.DNSResolverSettings      { return s.config }
func (s *emptyStore) GetRecords() []dnsrecords.DNSRecord                 { return nil }
func (s *emptyStore) GetCacheRecords() []dnsrecordcache.CacheRecord      { return nil }
func (s *emptyStore) LookupLocalRRs(string, string, bool) []dns.RR       { return nil }
func (s *emptyStore) LookupCacheRR(string, string) *dns.RR               { return nil }
func (s *emptyStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord)  {}
func (s *emptyStore) GetServers() []dnsservers.DNSServer                 { return nil }
func (s *emptyStore) GetBlockList() *adblock.BlockList                   { return adblock.NewBlockList() }
func (s *emptyStore) IncrementCacheHits()                                {}
func (s *emptyStore) IncrementQueriesAnswered()                          {}
func (s *emptyStore) IncrementTotalBlocks()                              {}
func (s *emptyStore) HasAnyLocalRecords() bool                           { return false }
func (s *emptyStore) HasAnyCachedRecords() bool                          { return false }
func (s *emptyStore) FilterHealthyUpstreamAddresses(a []string) []string { return a }
func (s *emptyStore) RecordUpstreamForwardSuccess(string)                {}
func (s *emptyStore) TryFastLocalOrCache(string, string, bool) (bool, []dns.RR, *dns.RR, bool) {
	return false, nil, nil, false
}

// TestResolver_NoLocalCacheUpstream_ReturnsEmpty verifies that when the store has no local
// records, no cache, and no upstream servers (and no fallback), the resolver returns
// an empty response (no answer). Integration test for no local/cache/upstream path.
func TestResolver_NoLocalCacheUpstream_ReturnsEmpty(t *testing.T) {
	store := &emptyStore{config: config.Config{}}
	rec := &recordingUpstream{}
	r := New(Config{
		Store:           store,
		Upstream:        rec,
		UpstreamTimeout: 2 * time.Second,
	})
	ctx := context.Background()

	question := dns.Question{Name: "unknown.example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	response := &dns.Msg{}
	response.SetQuestion(question.Name, question.Qtype)
	r.HandleQuestion(ctx, question, response)

	if len(response.Answer) != 0 {
		t.Errorf("expected no answers when no local/cache/upstream, got %d", len(response.Answer))
	}
	// Upstream should never be called (no servers configured)
	got := rec.recorded()
	if len(got) != 0 {
		t.Errorf("expected no upstream queries (no servers), got %d", len(got))
	}
}

// upstreamOnlyStore: no local records/cache, one global upstream (for fast-path QTYPE tests).
type upstreamOnlyStore struct {
	servers []dnsservers.DNSServer
	config  config.Config
}

func (s *upstreamOnlyStore) GetResolverSettings() data.DNSResolverSettings      { return s.config }
func (s *upstreamOnlyStore) GetRecords() []dnsrecords.DNSRecord                 { return nil }
func (s *upstreamOnlyStore) GetCacheRecords() []dnsrecordcache.CacheRecord      { return nil }
func (s *upstreamOnlyStore) LookupLocalRRs(string, string, bool) []dns.RR       { return nil }
func (s *upstreamOnlyStore) LookupCacheRR(string, string) *dns.RR               { return nil }
func (s *upstreamOnlyStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord)  {}
func (s *upstreamOnlyStore) GetServers() []dnsservers.DNSServer                 { return s.servers }
func (s *upstreamOnlyStore) GetBlockList() *adblock.BlockList                   { return adblock.NewBlockList() }
func (s *upstreamOnlyStore) IncrementCacheHits()                                {}
func (s *upstreamOnlyStore) IncrementQueriesAnswered()                          {}
func (s *upstreamOnlyStore) IncrementTotalBlocks()                              {}
func (s *upstreamOnlyStore) HasAnyLocalRecords() bool                           { return false }
func (s *upstreamOnlyStore) HasAnyCachedRecords() bool                          { return false }
func (s *upstreamOnlyStore) FilterHealthyUpstreamAddresses(a []string) []string { return a }
func (s *upstreamOnlyStore) RecordUpstreamForwardSuccess(string)                {}
func (s *upstreamOnlyStore) TryFastLocalOrCache(string, string, bool) (bool, []dns.RR, *dns.RR, bool) {
	return false, nil, nil, false
}

func TestResolver_AAAA_upstreamFastPath(t *testing.T) {
	srv := dnsservers.DNSServer{Address: "8.8.8.8", Port: "53", Active: true}
	store := &upstreamOnlyStore{servers: []dnsservers.DNSServer{srv}, config: config.Config{}}
	rec := &recordingUpstream{}
	r := New(Config{Store: store, Upstream: rec, UpstreamTimeout: 2 * time.Second})
	q := dns.Question{Name: "wide.example.com.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	msg := &dns.Msg{}
	msg.SetQuestion(q.Name, q.Qtype)
	r.HandleQuestion(context.Background(), q, msg)
	if len(msg.Answer) != 1 {
		t.Fatalf("want 1 answer, got %d", len(msg.Answer))
	}
	if _, ok := msg.Answer[0].(*dns.AAAA); !ok {
		t.Fatalf("want AAAA, got %T", msg.Answer[0])
	}
	if len(rec.recorded()) != 1 || rec.recorded()[0].server != "8.8.8.8:53" {
		t.Fatalf("upstream: %+v", rec.recorded())
	}
}

func TestResolver_MX_upstreamFastPath(t *testing.T) {
	srv := dnsservers.DNSServer{Address: "1.1.1.1", Port: "53", Active: true}
	store := &upstreamOnlyStore{servers: []dnsservers.DNSServer{srv}, config: config.Config{}}
	rec := &recordingUpstream{}
	r := New(Config{Store: store, Upstream: rec, UpstreamTimeout: 2 * time.Second})
	q := dns.Question{Name: "mail.example.com.", Qtype: dns.TypeMX, Qclass: dns.ClassINET}
	msg := &dns.Msg{}
	msg.SetQuestion(q.Name, q.Qtype)
	r.HandleQuestion(context.Background(), q, msg)
	if len(msg.Answer) != 1 {
		t.Fatalf("want 1 answer, got %d", len(msg.Answer))
	}
	if _, ok := msg.Answer[0].(*dns.MX); !ok {
		t.Fatalf("want MX, got %T", msg.Answer[0])
	}
}

func TestResolver_PTR_miss_usesUpstreamFastPath(t *testing.T) {
	srv := dnsservers.DNSServer{Address: "9.9.9.9", Port: "53", Active: true}
	store := &upstreamOnlyStore{servers: []dnsservers.DNSServer{srv}, config: config.Config{}}
	rec := &recordingUpstream{}
	r := New(Config{Store: store, Upstream: rec, UpstreamTimeout: 2 * time.Second})
	q := dns.Question{Name: "4.3.2.1.in-addr.arpa.", Qtype: dns.TypePTR, Qclass: dns.ClassINET}
	msg := &dns.Msg{}
	msg.SetQuestion(q.Name, q.Qtype)
	r.HandleQuestion(context.Background(), q, msg)
	if len(msg.Answer) != 1 {
		t.Fatalf("want 1 PTR answer, got %d", len(msg.Answer))
	}
	ptr, ok := msg.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("want PTR, got %T", msg.Answer[0])
	}
	if ptr.Ptr != "resolved.example.com." {
		t.Errorf("PTR target = %q", ptr.Ptr)
	}
	if len(rec.recorded()) < 1 {
		t.Fatal("expected upstream query for PTR miss")
	}
}
