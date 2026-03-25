// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"log/slog"
	"time"

	"dnsplane/data"
)

// runCacheCompactLoop removes expired cache rows on a timer; interval/enable from current config.
func runCacheCompactLoop(dnsData *data.DNSResolverData, lg *slog.Logger) {
	for {
		cfg := dnsData.GetResolverSettings()
		if !cfg.CacheRecords || !cfg.CacheCompactEnabled {
			dnsData.SetNextCacheCompactAt(time.Time{})
			time.Sleep(30 * time.Second)
			continue
		}
		sec := cfg.CacheCompactIntervalSeconds
		if sec < 60 {
			sec = 1800
		}
		interval := time.Duration(sec) * time.Second
		deadline := time.Now().Add(interval)
		dnsData.SetNextCacheCompactAt(deadline.UTC())

		sleep := time.Until(deadline)
		if sleep < 0 {
			sleep = interval
		}
		time.Sleep(sleep)

		// Re-read settings after sleep (may have been disabled).
		cfg = dnsData.GetResolverSettings()
		if !cfg.CacheRecords || !cfg.CacheCompactEnabled {
			dnsData.SetNextCacheCompactAt(time.Time{})
			continue
		}

		now := time.Now()
		removed := dnsData.CompactExpiredCacheRecords(now)
		remaining := dnsData.CacheRecordCount()
		dnsData.NoteCacheCompactRun(removed)
		if lg != nil {
			if removed > 0 {
				lg.Info("cache compact completed",
					"removed", removed,
					"remaining", remaining,
					"interval_seconds", cfg.CacheCompactIntervalSeconds,
				)
			} else {
				lg.Debug("cache compact completed",
					"removed", 0,
					"remaining", remaining,
					"interval_seconds", cfg.CacheCompactIntervalSeconds,
				)
			}
		}
	}
}
