package main

import (
	"log"
	"net"

	"github.com/miekg/dns"
)

// This is JUST a test, it will always return the same IP :P
func handleMDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	log.Printf("Received mDNS query: %s %s\n", q.Name, dns.TypeToString[q.Qtype])

	// Check if the request is for the .local domain
	if q.Qclass != dns.ClassINET || !dns.IsSubDomain("local.", q.Name) {
		log.Printf("Not an mDNS query, ignoring: %s\n", q.Name)
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch q.Qtype {
	case dns.TypeA:
		// Return IPv4 address for A query
		ipv4 := net.ParseIP("127.0.0.1")
		if ipv4 == nil {
			log.Printf("Invalid IPv4 address provided\n")
			return
		}
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
			A:   ipv4,
		})
	case dns.TypeAAAA:
		// Return IPv6 address for AAAA query
		ipv6 := net.ParseIP("::1")
		if ipv6 == nil {
			log.Printf("Invalid IPv6 address provided\n")
			return
		}
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 120},
			AAAA: ipv6,
		})
	default:
		log.Printf("Unsupported mDNS query type: %d\n", q.Qtype)
		return
	}

	// Write the response to the multicast address
	if err := w.WriteMsg(m); err != nil {
		log.Printf("Failed to write mDNS response: %v\n", err)
	}
}
