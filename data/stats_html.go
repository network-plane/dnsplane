// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

// StatsPageHTMLEnabled reports whether GET /stats/page is allowed (config stats_page_enabled, default true).
// Uses live resolver settings when initialised so TUI "server set" applies before save.
// When config is not loaded yet, returns true so defaults match legacy behaviour.
func StatsPageHTMLEnabled() bool {
	if instance != nil {
		return instance.GetResolverSettings().StatsPageEnabled
	}
	configStateMu.RLock()
	defer configStateMu.RUnlock()
	if configState == nil {
		return true
	}
	return configState.Config.StatsPageEnabled
}

// StatsPerfPageHTMLEnabled reports whether GET /stats/perf/page is allowed (stats_perf_page_enabled).
func StatsPerfPageHTMLEnabled() bool {
	if instance != nil {
		return instance.GetResolverSettings().StatsPerfPageEnabled
	}
	configStateMu.RLock()
	defer configStateMu.RUnlock()
	if configState == nil {
		return true
	}
	return configState.Config.StatsPerfPageEnabled
}

// StatsDashboardHTMLEnabled reports whether GET /stats/dashboard and /stats/dashboard/data are allowed.
func StatsDashboardHTMLEnabled() bool {
	if instance != nil {
		return instance.GetResolverSettings().StatsDashboardEnabled
	}
	configStateMu.RLock()
	defer configStateMu.RUnlock()
	if configState == nil {
		return true
	}
	return configState.Config.StatsDashboardEnabled
}
