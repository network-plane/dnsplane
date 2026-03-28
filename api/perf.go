// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"encoding/json"
	"net/http"

	"dnsplane/data"
)

func perfStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rep := data.GetResolverPerfReport()
	writeJSON(w, http.StatusOK, rep)
}

func perfResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	data.ResetResolverPerf()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "Resolver perf counters reset"})
}

// buildPerfReportPayload returns the same JSON shape as GET /stats/perf (for WebSocket push).
func buildPerfReportPayload() map[string]any {
	rep := data.GetResolverPerfReport()
	b, err := json.Marshal(rep)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return m
}
