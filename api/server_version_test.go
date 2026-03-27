// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	SetAppVersion("test-version")
	os.Exit(m.Run())
}

func TestVersionHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	versionHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var m map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"version", "go_version", "os", "arch"} {
		if m[k] == "" {
			t.Fatalf("missing or empty %q in %v", k, m)
		}
	}
}

func TestVersionPageHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version/page", nil)
	rec := httptest.NewRecorder()
	versionPageHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); ct == "" || !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	for _, s := range []string{"Build", "test-version", "Version", "</html>"} {
		if !strings.Contains(body, s) {
			t.Fatalf("expected %q in HTML body", s)
		}
	}
}
