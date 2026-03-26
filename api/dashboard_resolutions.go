// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"

	"dnsplane/data"
)

// dashboardResolutionsHandler returns the last in-memory resolution rows (newest first) for the dashboard grid.
func dashboardResolutionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	list := data.GetDashboardLogNewestFirst(data.DashboardLogCap())
	writeJSON(w, http.StatusOK, map[string]any{
		"cap":         data.DashboardLogCap(),
		"count":       len(list),
		"resolutions": list,
	})
}
