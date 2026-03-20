// Package dnssecsign signs authoritative answers from local data using BIND-style DNSKEY + private key files.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnssecsign

import (
	"bytes"
	"crypto"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Signer holds a zone ZSK and signs RRsets for names under that zone when the client sets DO (DNSSEC OK).
type Signer struct {
	zone string
	key  *dns.DNSKEY
	priv crypto.Signer
}

// LoadSigner reads a public DNSKEY (.key) and private key (.private) in BIND format.
func LoadSigner(zone, keyFile, privateKeyFile string) (*Signer, error) {
	zone = dns.Fqdn(strings.TrimSpace(zone))
	if zone == "." {
		return nil, fmt.Errorf("dnssecsign: invalid zone")
	}
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("dnssecsign: read key file: %w", err)
	}
	privBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("dnssecsign: read private key file: %w", err)
	}
	rr, err := dns.NewRR(strings.TrimSpace(string(keyBytes)))
	if err != nil {
		return nil, fmt.Errorf("dnssecsign: parse DNSKEY: %w", err)
	}
	dk, ok := rr.(*dns.DNSKEY)
	if !ok {
		return nil, fmt.Errorf("dnssecsign: %s does not contain a DNSKEY record", keyFile)
	}
	priv, err := dk.ReadPrivateKey(bytes.NewReader(privBytes), privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("dnssecsign: parse private key: %w", err)
	}
	cs, ok := priv.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("dnssecsign: private key does not implement crypto.Signer")
	}
	return &Signer{zone: dns.Fqdn(zone), key: dk, priv: cs}, nil
}

// Zone returns the signed zone apex (FQDN).
func (s *Signer) Zone() string {
	if s == nil {
		return ""
	}
	return s.zone
}

// CoversName reports whether qname is the zone apex or a name under the zone.
func (s *Signer) CoversName(qname string) bool {
	if s == nil {
		return false
	}
	qname = dns.Fqdn(qname)
	return dns.IsSubDomain(s.zone, qname)
}

// SignLocalAnswerIfDO appends RRSIG (and DNSKEY in Extra) when the client sent DO and the RRset is signable.
func (s *Signer) SignLocalAnswerIfDO(req *dns.Msg, q dns.Question, rrset []dns.RR, resp *dns.Msg) {
	if s == nil || req == nil || resp == nil || len(rrset) == 0 {
		return
	}
	opt := req.IsEdns0()
	if opt == nil || !opt.Do() {
		return
	}
	if !s.CoversName(q.Name) {
		return
	}
	if !dns.IsRRset(rrset) {
		return
	}
	ttl := rrset[0].Header().Ttl
	now := uint32(time.Now().Unix())
	sig := &dns.RRSIG{
		Hdr:        dns.RR_Header{Rrtype: dns.TypeRRSIG, Class: q.Qclass, Ttl: ttl},
		Algorithm:  s.key.Algorithm,
		Expiration: now + 30*86400,
		Inception:  now - 3600,
		KeyTag:     s.key.KeyTag(),
		SignerName: s.zone,
	}
	if err := sig.Sign(s.priv, rrset); err != nil {
		return
	}
	resp.Answer = append(resp.Answer, sig)
	if !hasDNSKEYForZone(resp.Extra, s.zone) {
		dk := &dns.DNSKEY{}
		*dk = *s.key
		dk.Hdr = dns.RR_Header{
			Name:   s.zone,
			Rrtype: dns.TypeDNSKEY,
			Class:  dns.ClassINET,
			Ttl:    3600,
		}
		resp.Extra = append(resp.Extra, dk)
	}
	resp.AuthenticatedData = true
}

func hasDNSKEYForZone(extra []dns.RR, zone string) bool {
	zone = dns.Fqdn(zone)
	for _, rr := range extra {
		if k, ok := rr.(*dns.DNSKEY); ok && dns.Fqdn(k.Hdr.Name) == zone {
			return true
		}
	}
	return false
}
