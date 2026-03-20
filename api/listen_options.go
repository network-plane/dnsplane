// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

// ListenOptions configures bind address, TLS, and per-IP rate limiting for the REST API.
type ListenOptions struct {
	BindIP         string
	TLSCertFile    string
	TLSKeyFile     string
	RateLimitRPS   float64
	RateLimitBurst int
}
