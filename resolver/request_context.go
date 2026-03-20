// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package resolver

import (
	"context"

	"github.com/miekg/dns"
)

type requestCtxKey struct{}

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
