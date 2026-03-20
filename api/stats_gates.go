// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import "net/http"

// requireStatsHTMLPage returns false and sends 404 when a stats HTML route is disabled in config.
func requireStatsHTMLPage(w http.ResponseWriter, r *http.Request, enabled bool) bool {
	if enabled {
		return true
	}
	http.NotFound(w, r)
	return false
}
