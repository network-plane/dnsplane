// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardPageHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats/dashboard", nil)
	rec := httptest.NewRecorder()
	dashboardPageHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "dnsplane") {
		t.Fatal("expected dashboard HTML body")
	}
}
