// Package zones parses BIND-style zone files into dnsplane dnsrecords.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package zones

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"dnsplane/dnsrecords"

	"github.com/miekg/dns"
)

// ParseResult is the outcome of parsing one or more zone files.
type ParseResult struct {
	Records  []dnsrecords.DNSRecord
	Warnings []string
}

// ParseFile reads a zone file from path and converts supported RRs to DNSRecord.
func ParseFile(path string) (ParseResult, error) {
	f, err := os.Open(path) // #nosec G304 -- path is caller-supplied zone file argument
	if err != nil {
		return ParseResult{}, err
	}
	defer func() { _ = f.Close() }()
	return ParseReader(f, path)
}

// ParseReader parses zone data from r. fileHint is used for parser diagnostics ($INCLUDE resolution).
func ParseReader(r io.Reader, fileHint string) (ParseResult, error) {
	zp := dns.NewZoneParser(r, "", fileHint)
	zp.SetIncludeAllowed(false)
	var out ParseResult
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		if rr == nil {
			continue
		}
		rec, okConv, warn := rrToRecord(rr)
		if warn != "" {
			out.Warnings = append(out.Warnings, warn)
		}
		if !okConv {
			continue
		}
		out.Records = append(out.Records, rec)
	}
	if err := zp.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func rrToRecord(rr dns.RR) (rec dnsrecords.DNSRecord, ok bool, warn string) {
	h := rr.Header()
	name := dnsrecords.CanonicalizeRecordNameForStorage(h.Name)
	ttl := h.Ttl
	typ := dns.TypeToString[h.Rrtype]
	if typ == "" {
		return dnsrecords.DNSRecord{}, false, fmt.Sprintf("skip unknown RR type %d at %s", h.Rrtype, name)
	}

	switch v := rr.(type) {
	case *dns.A:
		if v.A == nil {
			return dnsrecords.DNSRecord{}, false, ""
		}
		return dnsrecords.DNSRecord{Name: name, Type: "A", Value: v.A.String(), TTL: ttl}, true, ""
	case *dns.AAAA:
		if v.AAAA == nil {
			return dnsrecords.DNSRecord{}, false, ""
		}
		return dnsrecords.DNSRecord{Name: name, Type: "AAAA", Value: v.AAAA.String(), TTL: ttl}, true, ""
	case *dns.CNAME:
		return dnsrecords.DNSRecord{Name: name, Type: "CNAME", Value: strings.TrimSpace(v.Target), TTL: ttl}, true, ""
	case *dns.NS:
		return dnsrecords.DNSRecord{Name: name, Type: "NS", Value: strings.TrimSpace(v.Ns), TTL: ttl}, true, ""
	case *dns.PTR:
		return dnsrecords.DNSRecord{Name: name, Type: "PTR", Value: strings.TrimSpace(v.Ptr), TTL: ttl}, true, ""
	case *dns.MX:
		val := fmt.Sprintf("%d %s", v.Preference, strings.TrimSpace(v.Mx))
		return dnsrecords.DNSRecord{Name: name, Type: "MX", Value: val, TTL: ttl}, true, ""
	case *dns.TXT:
		if len(v.Txt) == 0 {
			return dnsrecords.DNSRecord{}, false, ""
		}
		parts := make([]string, 0, len(v.Txt))
		for _, s := range v.Txt {
			parts = append(parts, strconv.Quote(s))
		}
		val := strings.Join(parts, " ")
		return dnsrecords.DNSRecord{Name: name, Type: "TXT", Value: val, TTL: ttl}, true, ""
	case *dns.SOA:
		val := fmt.Sprintf("%s %s %d %d %d %d %d",
			strings.TrimSpace(v.Ns),
			strings.TrimSpace(v.Mbox),
			v.Serial,
			v.Refresh,
			v.Retry,
			v.Expire,
			v.Minttl,
		)
		return dnsrecords.DNSRecord{Name: name, Type: "SOA", Value: val, TTL: ttl}, true, ""
	default:
		return dnsrecords.DNSRecord{}, false, fmt.Sprintf("skip unsupported type %s at %s", typ, name)
	}
}
