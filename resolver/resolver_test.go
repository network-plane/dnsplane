package resolver

import (
	"context"
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
	msg.MsgHdr.Rcode = dns.RcodeSuccess
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   []byte{1, 2, 3, 4},
		},
	}
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

func (s *whitelistIntegrationStore) GetResolverSettings() data.DNSResolverSettings     { return s.config }
func (s *whitelistIntegrationStore) GetRecords() []dnsrecords.DNSRecord                { return nil }
func (s *whitelistIntegrationStore) GetCacheRecords() []dnsrecordcache.CacheRecord     { return nil }
func (s *whitelistIntegrationStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord) {}
func (s *whitelistIntegrationStore) GetServers() []dnsservers.DNSServer                { return s.servers }
func (s *whitelistIntegrationStore) GetBlockList() *adblock.BlockList {
	return adblock.NewBlockList()
}
func (s *whitelistIntegrationStore) IncrementCacheHits()       {}
func (s *whitelistIntegrationStore) IncrementQueriesAnswered() {}
func (s *whitelistIntegrationStore) IncrementTotalBlocks()     {}

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

func (s *localRecordStore) GetResolverSettings() data.DNSResolverSettings     { return s.config }
func (s *localRecordStore) GetRecords() []dnsrecords.DNSRecord                { return s.records }
func (s *localRecordStore) GetCacheRecords() []dnsrecordcache.CacheRecord     { return nil }
func (s *localRecordStore) UpdateCacheRecords(_ []dnsrecordcache.CacheRecord) {}
func (s *localRecordStore) GetServers() []dnsservers.DNSServer                { return nil }
func (s *localRecordStore) GetBlockList() *adblock.BlockList                  { return adblock.NewBlockList() }
func (s *localRecordStore) IncrementCacheHits()                               {}
func (s *localRecordStore) IncrementQueriesAnswered()                         {}
func (s *localRecordStore) IncrementTotalBlocks()                             {}

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
