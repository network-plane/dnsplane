// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"log/slog"
	"os"
	"sync/atomic"
)

var resolverLog atomic.Pointer[slog.Logger]

// SetResolverLogger sets the logger for resolver-side messages (adblock, record refresh, persist, fatals).
// Wire before InitializeJSONFiles/GetInstance when using a dedicated dnsserver log. Nil uses slog.Default().
func SetResolverLogger(lg *slog.Logger) {
	resolverLog.Store(lg)
}

func resolverSlog() *slog.Logger {
	if lg := resolverLog.Load(); lg != nil {
		return lg
	}
	return slog.Default()
}

func fatalResolver(msg string, args ...any) {
	resolverSlog().Error(msg, args...)
	os.Exit(1)
}
