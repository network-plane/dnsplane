// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"log/slog"
	"time"

	"dnsplane/data"
)

// runCacheCompactLoop removes expired cache rows on a timer; interval/enable from current config.
// A manual `cache compact` calls BumpCacheCompactScheduleAfterManual so the next automatic run is a full interval later.
func runCacheCompactLoop(dnsData *data.DNSResolverData, lg *slog.Logger) {
	bump := dnsData.CacheCompactBumpReceiver()
outer:
	for {
		cfg := dnsData.GetResolverSettings()
		if !cfg.CacheRecords || !cfg.CacheCompactEnabled {
			dnsData.SetNextCacheCompactAt(time.Time{})
			time.Sleep(30 * time.Second)
			continue
		}
		interval := data.CacheCompactInterval(cfg.CacheCompactIntervalSeconds)

	inner:
		for {
			deadline := time.Now().Add(interval)
			dnsData.SetNextCacheCompactAt(deadline.UTC())

			wait := time.Until(deadline)
			if wait < 0 {
				wait = interval
			}
			timer := time.NewTimer(wait)
			if bump == nil {
				<-timer.C
				break inner
			}
			select {
			case <-timer.C:
				break inner
			case <-bump:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				cfg = dnsData.GetResolverSettings()
				if !cfg.CacheRecords || !cfg.CacheCompactEnabled {
					dnsData.SetNextCacheCompactAt(time.Time{})
					continue outer
				}
				interval = data.CacheCompactInterval(cfg.CacheCompactIntervalSeconds)
				continue inner
			}
		}

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
