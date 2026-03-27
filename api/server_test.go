// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("healthHandler status = %d, want 200", rec.Code)
	}
}

func TestListRecordsQueryDetails(t *testing.T) {
	if !listRecordsQueryDetails(url.Values{"details": []string{"1"}}) {
		t.Fatal("details=1 should be true")
	}
	if !listRecordsQueryDetails(url.Values{"details": []string{"true"}}) {
		t.Fatal("details=true should be true")
	}
	if !listRecordsQueryDetails(url.Values{"d": []string{"1"}}) {
		t.Fatal("d=1 should be true")
	}
	if listRecordsQueryDetails(url.Values{"details": []string{"0"}}) {
		t.Fatal("details=0 should be false")
	}
}

func TestListRecordsFilterDesc(t *testing.T) {
	if got := listRecordsFilterDesc("a", "A"); got != "name=a&type=A" {
		t.Fatalf("got %q", got)
	}
	if got := listRecordsFilterDesc("a", ""); got != "name=a" {
		t.Fatalf("got %q", got)
	}
	if got := listRecordsFilterDesc("", "MX"); got != "type=MX" {
		t.Fatalf("got %q", got)
	}
}

func TestReadyHandler_WithoutState(t *testing.T) {
	// With apiState nil, ready should report not ready
	apiServerMu.Lock()
	old := apiState
	apiState = nil
	apiServerMu.Unlock()
	defer func() {
		apiServerMu.Lock()
		apiState = old
		apiServerMu.Unlock()
	}()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	readyHandler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyHandler with nil state status = %d, want 503", rec.Code)
	}
}
