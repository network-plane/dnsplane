// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package dnsserve

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"dnsplane/dnsrecords"

	"github.com/miekg/dns"
)

// tryServeAXFR handles AXFR over TCP/DoT when enabled and authorized.
func tryServeAXFR(req *dns.Msg, meta ServeMeta, dep Dependencies) (*dns.Msg, bool) {
	if req.Opcode != dns.OpcodeQuery || len(req.Question) != 1 {
		return nil, false
	}
	if req.Question[0].Qtype != dns.TypeAXFR {
		return nil, false
	}
	proto := strings.TrimSpace(meta.Protocol)
	if proto != ProtoTCP && proto != ProtoDoT {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.SetRcode(req, dns.RcodeNotImplemented)
		return resp, true
	}

	st := dep.Settings()
	if !st.AXFREnabled {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.SetRcode(req, dns.RcodeRefused)
		return resp, true
	}
	nets, err := parseCIDRs(st.AXFRAllowedNetworks)
	if err != nil || len(nets) == 0 {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.SetRcode(req, dns.RcodeRefused)
		return resp, true
	}
	if !clientIPAllowed(meta.ClientIP, nets) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.SetRcode(req, dns.RcodeRefused)
		return resp, true
	}

	zone := dns.CanonicalName(req.Question[0].Name)
	var recs []dnsrecords.DNSRecord
	if dep.LocalRecords != nil {
		recs = dep.LocalRecords()
	}
	rrs, err := axfrRecordsForZone(recs, zone)
	if err != nil {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.SetRcode(req, dns.RcodeRefused)
		return resp, true
	}

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true
	resp.Answer = rrs
	return resp, true
}

func parseCIDRs(ss []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func clientIPAllowed(ip string, nets []*net.IPNet) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "unknown" {
		return false
	}
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(addr) {
			return true
		}
	}
	return false
}

func axfrRecordsForZone(recs []dnsrecords.DNSRecord, zone string) ([]dns.RR, error) {
	var inZone []dnsrecords.DNSRecord
	for _, r := range recs {
		n := dns.CanonicalName(r.Name)
		if dns.IsSubDomain(zone, n) {
			inZone = append(inZone, r)
		}
	}
	var soa dnsrecords.DNSRecord
	var foundSOA bool
	for _, r := range inZone {
		if strings.EqualFold(strings.TrimSpace(r.Type), "SOA") {
			soa = r
			foundSOA = true
			break
		}
	}
	if !foundSOA {
		return nil, fmt.Errorf("no SOA in zone")
	}
	var others []dnsrecords.DNSRecord
	for _, r := range inZone {
		if strings.EqualFold(strings.TrimSpace(r.Type), "SOA") {
			continue
		}
		others = append(others, r)
	}
	sort.Slice(others, func(i, j int) bool {
		a, b := others[i], others[j]
		ka := dnsrecords.NormalizeRecordNameKey(a.Name) + "|" + dnsrecords.NormalizeRecordType(a.Type)
		kb := dnsrecords.NormalizeRecordNameKey(b.Name) + "|" + dnsrecords.NormalizeRecordType(b.Type)
		if ka != kb {
			return ka < kb
		}
		return a.Value < b.Value
	})

	out := make([]dns.RR, 0, len(others)+2)
	first, err := dnsRecordToRR(soa)
	if err != nil {
		return nil, err
	}
	out = append(out, first)
	for _, r := range others {
		rr, err := dnsRecordToRR(r)
		if err != nil {
			continue
		}
		out = append(out, rr)
	}
	last, err := dnsRecordToRR(soa)
	if err != nil {
		return nil, err
	}
	out = append(out, last)
	return out, nil
}

func dnsRecordToRR(rec dnsrecords.DNSRecord) (dns.RR, error) {
	name := strings.TrimSpace(rec.Name)
	if name == "" {
		return nil, fmt.Errorf("empty name")
	}
	name = dns.CanonicalName(name)
	ttl := rec.TTL
	if ttl == 0 {
		ttl = 3600
	}
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	val := strings.TrimSpace(rec.Value)
	switch typ {
	case "NS", "CNAME", "PTR":
		if val != "" {
			val = dns.CanonicalName(val)
		}
	case "MX":
		parts := strings.SplitN(val, " ", 2)
		if len(parts) == 2 {
			val = strings.TrimSpace(parts[0]) + " " + dns.CanonicalName(strings.TrimSpace(parts[1]))
		}
	}
	line := fmt.Sprintf("%s %d IN %s %s", name, ttl, typ, val)
	return dns.NewRR(line)
}
