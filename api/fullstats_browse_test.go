// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"dnsplane/fullstats"
)

func TestParseFullStatsBrowseParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats/dashboard/fullstats/data?scope=session&table=requesters&sort=ip_asc&page=2&per_page=50", nil)
	p, err := parseFullStatsBrowseParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Scope != "session" || p.Table != "requesters" || p.Sort != "ip_asc" || p.Page != 2 || p.PerPage != 50 || p.Query != "" {
		t.Fatalf("unexpected params: %+v", p)
	}
	reqQ := httptest.NewRequest(http.MethodGet, "/x?q=amazonvideo&table=requests", nil)
	pq, err := parseFullStatsBrowseParams(reqQ)
	if err != nil || pq.Query != "amazonvideo" || pq.Table != "requests" {
		t.Fatalf("q param: %+v err %v", pq, err)
	}
	long := strings.Repeat("x", fullStatsBrowseMaxQuery+1)
	reqLong := httptest.NewRequest(http.MethodGet, "/x?q="+url.QueryEscape(long), nil)
	if _, err := parseFullStatsBrowseParams(reqLong); err == nil {
		t.Fatal("expected error for q too long")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	p2, err := parseFullStatsBrowseParams(req2)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Scope != "total" || p2.Table != "requests" || p2.Sort != "count_desc" || p2.Page != 1 || p2.PerPage != fullStatsBrowseDefaultPer {
		t.Fatalf("defaults: %+v", p2)
	}

	bad := httptest.NewRequest(http.MethodGet, "/x?scope=nope", nil)
	if _, err := parseFullStatsBrowseParams(bad); err == nil {
		t.Fatal("expected error for bad scope")
	}
}

func TestParseFullStatsBrowseParams_TableQuery(t *testing.T) {
	u, _ := url.Parse("/stats/dashboard/fullstats/data")
	u.RawQuery = url.Values{"table": {"requests"}, "sort": {"key_asc"}}.Encode()
	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	p, err := parseFullStatsBrowseParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Table != "requests" || p.Sort != "key_asc" {
		t.Fatalf("%+v", p)
	}
	u2, _ := url.Parse("/stats/dashboard/fullstats/data")
	u2.RawQuery = url.Values{"table": {"requests"}, "sort": {"type_desc"}}.Encode()
	req2 := httptest.NewRequest(http.MethodGet, u2.String(), nil)
	p2, err := parseFullStatsBrowseParams(req2)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Sort != "type_desc" {
		t.Fatalf("type_desc: %+v", p2)
	}
}

func TestPaginateSlice(t *testing.T) {
	rows := []int{1, 2, 3, 4, 5}
	page, tp, slice := paginateSlice(rows, 2, 2)
	if page != 2 || tp != 3 || len(slice) != 2 || slice[0] != 3 {
		t.Fatalf("page=%d tp=%d slice=%v", page, tp, slice)
	}
	page, tp, slice = paginateSlice([]int{}, 5, 25)
	if page != 1 || tp != 1 || slice != nil {
		t.Fatalf("empty: page=%d tp=%d slice=%v", page, tp, slice)
	}
	page, tp, slice = paginateSlice(rows, 99, 2)
	if page != 3 || tp != 3 || len(slice) != 1 || slice[0] != 5 {
		t.Fatalf("clamp page: page=%d tp=%d slice=%v", page, tp, slice)
	}
}

func TestFilterRequestRows(t *testing.T) {
	rows := []requestRowSortable{
		{key: "a.com:A", st: &fullstats.RequestStats{Count: 1}},
		{key: "b.in-addr.arpa.:PTR", st: &fullstats.RequestStats{Count: 2}},
	}
	out := filterRequestRows(rows, "ptr")
	if len(out) != 1 || out[0].key != "b.in-addr.arpa.:PTR" {
		t.Fatalf("got %+v", out)
	}
	if len(filterRequestRows(rows, "")) != 2 {
		t.Fatal("empty q should keep all")
	}
}

func TestFilterRequesterRows(t *testing.T) {
	rows := []requesterRowSortable{
		{ip: "192.168.1.1", st: &fullstats.RequesterStats{TypeCount: map[string]uint64{"A": 1}}, total: 1},
		{ip: "10.0.0.1", st: &fullstats.RequesterStats{TypeCount: map[string]uint64{"PTR": 5}}, total: 5},
	}
	out := filterRequesterRows(rows, "ptr")
	if len(out) != 1 || out[0].ip != "10.0.0.1" {
		t.Fatalf("got %+v", out)
	}
	out2 := filterRequesterRows(rows, "192")
	if len(out2) != 1 || out2[0].ip != "192.168.1.1" {
		t.Fatalf("got %+v", out2)
	}
}

func TestFullstatsBrowseHandler_Disabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats/dashboard/fullstats/data", nil)
	rec := httptest.NewRecorder()
	fullstatsBrowseHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatal(rec.Body.String())
	}
}
