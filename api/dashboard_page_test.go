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
	body := rec.Body.String()
	if !strings.Contains(body, "dnsplane") {
		t.Fatal("expected dashboard HTML body")
	}
	if !strings.Contains(body, "dash-icon-wrap") {
		t.Fatal("expected Tabler inline icons in dashboard markup")
	}
}

func TestDashboardIconSVGHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats/dashboard/icon?name=uptime", nil)
	rec := httptest.NewRecorder()
	dashboardIconSVGHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/svg+xml") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatal("expected svg body")
	}

	reqBad := httptest.NewRequest(http.MethodGet, "/stats/dashboard/icon?name=nope", nil)
	recBad := httptest.NewRecorder()
	dashboardIconSVGHandler(recBad, reqBad)
	if recBad.Code != http.StatusNotFound {
		t.Fatalf("unknown icon: status = %d", recBad.Code)
	}
}
