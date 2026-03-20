// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"
	"strings"
	"time"

	"github.com/miekg/dns"

	"dnsplane/dnsservers"
)

// DNSClient implements UpstreamClient using github.com/miekg/dns.
type DNSClient struct {
	client *dns.Client
}

// NewDNSClient creates an upstream client with the desired dial timeout.
func NewDNSClient(timeout time.Duration) *DNSClient {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &DNSClient{
		client: &dns.Client{
			Timeout: timeout,
		},
	}
}

func (c *DNSClient) clientFor(ep dnsservers.UpstreamEndpoint) *dns.Client {
	if c == nil || c.client == nil {
		return nil
	}
	cl := new(dns.Client)
	*cl = *c.client
	t := strings.ToLower(strings.TrimSpace(ep.Transport))
	if t == "" {
		t = "udp"
	}
	switch t {
	case "udp":
		cl.Net = "udp"
	case "tcp":
		cl.Net = "tcp"
	case "dot":
		cl.Net = "tcp-tls"
	case "doh":
		cl.Net = "https"
	default:
		cl.Net = "udp"
	}
	return cl
}

// Query sends the question to the specified upstream endpoint.
func (c *DNSClient) Query(ctx context.Context, question dns.Question, ep dnsservers.UpstreamEndpoint) (*dns.Msg, error) {
	cl := c.clientFor(ep)
	if cl == nil {
		return nil, nil
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)
	resp, _, err := cl.ExchangeContext(ctx, message, ep.Addr)
	return resp, err
}
