// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"
	"strings"

	"github.com/miekg/dns"
)

type requestCtxKey struct{}
type clientIPCtxKey struct{}

// ContextWithRequest attaches the original DNS query message (for DO bit, EDNS).
func ContextWithRequest(ctx context.Context, req *dns.Msg) context.Context {
	if req == nil {
		return ctx
	}
	return context.WithValue(ctx, requestCtxKey{}, req)
}

// RequestFromContext returns the original query message if present.
func RequestFromContext(ctx context.Context) *dns.Msg {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(requestCtxKey{}).(*dns.Msg)
	return v
}

// ContextWithClientIP attaches the client address string (from ServeMeta) for observers/stats.
func ContextWithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPCtxKey{}, strings.TrimSpace(ip))
}

// ClientIPFromContext returns the client IP set by ContextWithClientIP, or "".
func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientIPCtxKey{}).(string)
	return v
}
