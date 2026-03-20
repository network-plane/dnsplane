// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubtleStringEqual(t *testing.T) {
	if !subtleStringEqual("a", "a") {
		t.Fatal("equal strings")
	}
	if subtleStringEqual("a", "b") {
		t.Fatal("different bytes")
	}
	if subtleStringEqual("ab", "a") {
		t.Fatal("length mismatch")
	}
}

func TestApiAuthExempt(t *testing.T) {
	if !apiAuthExempt(httptest.NewRequest(http.MethodGet, "/health", nil)) {
		t.Fatal("GET /health")
	}
	if !apiAuthExempt(httptest.NewRequest(http.MethodHead, "/ready", nil)) {
		t.Fatal("HEAD /ready")
	}
	if apiAuthExempt(httptest.NewRequest(http.MethodPost, "/health", nil)) {
		t.Fatal("POST /health should not exempt")
	}
	if apiAuthExempt(httptest.NewRequest(http.MethodGet, "/dns/records", nil)) {
		t.Fatal("GET /dns/records not exempt")
	}
}

func TestApiRequestAuthorized(t *testing.T) {
	want := "secret-token"
	{
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer "+want)
		if !apiRequestAuthorized(r, want) {
			t.Fatal("Bearer")
		}
	}
	{
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-API-Token", want)
		if !apiRequestAuthorized(r, want) {
			t.Fatal("X-API-Token")
		}
	}
	{
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if apiRequestAuthorized(r, want) {
			t.Fatal("missing auth")
		}
	}
	{
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer wrong")
		if apiRequestAuthorized(r, want) {
			t.Fatal("wrong token")
		}
	}
}
