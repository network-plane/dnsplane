// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

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
