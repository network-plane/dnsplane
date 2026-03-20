// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

// Package buildinfo exposes release and runtime metadata for APIs and logging.
package buildinfo

import "runtime"

// Version is the dnsplane release string (bump for releases; may be overridden with -ldflags).
var Version = "1.3.103"

// Info returns version and runtime fields for JSON APIs.
func Info() map[string]string {
	return map[string]string{
		"version":    Version,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
}
