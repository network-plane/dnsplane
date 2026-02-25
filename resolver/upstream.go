// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"
	"time"

	"github.com/miekg/dns"
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

// Query sends the question to the specified DNS server.
func (c *DNSClient) Query(ctx context.Context, question dns.Question, server string) (*dns.Msg, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)
	resp, _, err := c.client.ExchangeContext(ctx, message, server)
	return resp, err
}
