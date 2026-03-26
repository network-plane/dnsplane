// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dnsplane/data"
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
	if !strings.Contains(body, "view-resolutions") {
		t.Fatal("expected resolutions log view in dashboard markup")
	}
}

func TestDashboardResolutionsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats/dashboard/resolutions", nil)
	rec := httptest.NewRecorder()
	dashboardResolutionsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	b := rec.Body.String()
	if !strings.Contains(b, `"resolutions"`) {
		t.Fatal("expected resolutions key in JSON body")
	}
}

func TestDashboardResolutionsPurgeHandler(t *testing.T) {
	data.RecordDashboardResolution(data.DashboardResolution{
		At: time.Now().UTC(), Qname: "purge.test.", Qtype: "A", Outcome: "cache", Record: "9.9.9.9", DurationMs: 0.5,
	})
	req := httptest.NewRequest(http.MethodPost, "/stats/dashboard/resolutions/purge", nil)
	rec := httptest.NewRecorder()
	dashboardResolutionsPurgeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("purge status = %d body=%s", rec.Code, rec.Body.String())
	}
	reqGet := httptest.NewRequest(http.MethodGet, "/stats/dashboard/resolutions", nil)
	recGet := httptest.NewRecorder()
	dashboardResolutionsHandler(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get after purge: status = %d", recGet.Code)
	}
	if !strings.Contains(recGet.Body.String(), `"count":0`) {
		t.Fatalf("expected empty log after purge: %s", recGet.Body.String())
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
