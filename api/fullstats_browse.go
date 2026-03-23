// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"dnsplane/data"
	"dnsplane/fullstats"
)

const (
	fullStatsBrowseMaxPerPage = 100
	fullStatsBrowseDefaultPer = 25
	fullStatsBrowseMaxQuery   = 256
)

// fullstatsBrowseHandler returns paginated full_stats (bbolt) or session data for the dashboard.
// Query: scope=total|session, table=requests|requesters, sort=..., page=1, per_page=25, q=substring (optional)
func fullstatsBrowseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	apiServerMu.Lock()
	tracker := apiFullStatsTracker
	apiServerMu.Unlock()
	if tracker == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"message": "full statistics tracking is not enabled",
		})
		return
	}

	params, err := parseFullStatsBrowseParams(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	switch params.Table {
	case "requests":
		rows, err := collectRequestRows(tracker, params.Scope)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		rows = filterRequestRows(rows, params.Query)
		sortRequestRows(rows, params.Sort)
		page, totalPages, slice := paginateSlice(rows, params.Page, params.PerPage)
		out := make([]fullStatsRequestRowJSON, len(slice))
		for i, row := range slice {
			domain, rtype := splitDomainType(row.key)
			var src map[string]uint64
			if row.st != nil && row.st.SourceCount != nil {
				src = make(map[string]uint64, len(row.st.SourceCount))
				for k, v := range row.st.SourceCount {
					src[k] = v
				}
			}
			out[i] = fullStatsRequestRowJSON{
				Key: row.key, Domain: domain, RecordType: rtype,
				Count:       row.st.Count,
				SourceCount: src,
				FirstSeen:   formatRFC3339(row.st.FirstSeen),
				LastSeen:    formatRFC3339(row.st.LastSeen),
			}
		}
		writeJSON(w, http.StatusOK, fullStatsBrowseResponseJSON{
			Enabled: true, Scope: params.Scope, Table: params.Table, Sort: params.Sort,
			Q:    params.Query,
			Page: page, PerPage: params.PerPage, TotalRows: len(rows), TotalPages: totalPages,
			Rows: out,
		})
	case "requesters":
		rows, err := collectRequesterRows(tracker, params.Scope)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		rows = filterRequesterRows(rows, params.Query)
		sortRequesterRows(rows, params.Sort)
		page, totalPages, slice := paginateSlice(rows, params.Page, params.PerPage)
		out := make([]fullStatsRequesterRowJSON, len(slice))
		for i, row := range slice {
			byType := map[string]uint64{}
			if row.st.TypeCount != nil {
				for k, v := range row.st.TypeCount {
					byType[k] = v
				}
			}
			bySrc := map[string]uint64{}
			if row.st.SourceCount != nil {
				for k, v := range row.st.SourceCount {
					bySrc[k] = v
				}
			}
			out[i] = fullStatsRequesterRowJSON{
				IP: row.ip, TotalRequests: row.total, FirstSeen: formatRFC3339(row.st.FirstSeen),
				ByType: byType, BySource: bySrc,
			}
		}
		writeJSON(w, http.StatusOK, fullStatsBrowseResponseJSON{
			Enabled: true, Scope: params.Scope, Table: params.Table, Sort: params.Sort,
			Q:    params.Query,
			Page: page, PerPage: params.PerPage, TotalRows: len(rows), TotalPages: totalPages,
			Rows: out,
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid table"})
	}
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type fullStatsBrowseParams struct {
	Scope   string
	Table   string
	Sort    string
	Query   string
	Page    int
	PerPage int
}

func parseFullStatsBrowseParams(r *http.Request) (fullStatsBrowseParams, error) {
	q := r.URL.Query()
	scope := strings.TrimSpace(strings.ToLower(q.Get("scope")))
	if scope == "" {
		scope = "total"
	}
	if scope != "total" && scope != "session" {
		return fullStatsBrowseParams{}, errInvalid("scope must be total or session")
	}
	table := strings.TrimSpace(strings.ToLower(q.Get("table")))
	if table == "" {
		table = "requests"
	}
	if table != "requests" && table != "requesters" {
		return fullStatsBrowseParams{}, errInvalid("table must be requests or requesters")
	}
	sortKey := strings.TrimSpace(strings.ToLower(q.Get("sort")))
	if sortKey == "" {
		sortKey = defaultSortForTable(table)
	}
	if !validSortForTable(table, sortKey) {
		return fullStatsBrowseParams{}, errInvalid("invalid sort for table")
	}
	page := 1
	if s := strings.TrimSpace(q.Get("page")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return fullStatsBrowseParams{}, errInvalid("page must be a positive integer")
		}
		page = n
	}
	perPage := fullStatsBrowseDefaultPer
	if s := strings.TrimSpace(q.Get("per_page")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > fullStatsBrowseMaxPerPage {
			return fullStatsBrowseParams{}, errInvalid("per_page must be between 1 and " + strconv.Itoa(fullStatsBrowseMaxPerPage))
		}
		perPage = n
	}
	searchQ := strings.TrimSpace(q.Get("q"))
	if len(searchQ) > fullStatsBrowseMaxQuery {
		return fullStatsBrowseParams{}, errInvalid("q must be at most " + strconv.Itoa(fullStatsBrowseMaxQuery) + " characters")
	}
	return fullStatsBrowseParams{Scope: scope, Table: table, Sort: sortKey, Query: searchQ, Page: page, PerPage: perPage}, nil
}

type invalidParamError string

func (e invalidParamError) Error() string { return string(e) }

func errInvalid(msg string) error {
	return invalidParamError(msg)
}

func defaultSortForTable(table string) string {
	switch table {
	case "requesters":
		return "total_desc"
	default:
		return "count_desc"
	}
}

func validSortForTable(table, sort string) bool {
	switch table {
	case "requests":
		switch sort {
		case "count_desc", "count_asc", "key_asc", "last_seen_desc", "last_seen_asc",
			"first_seen_desc", "first_seen_asc":
			return true
		}
	case "requesters":
		switch sort {
		case "total_desc", "total_asc", "ip_asc", "first_seen_desc", "first_seen_asc":
			return true
		}
	}
	return false
}

type requestRowSortable struct {
	key string
	st  *fullstats.RequestStats
}

func collectRequestRows(tracker *fullstats.Tracker, scope string) ([]requestRowSortable, error) {
	var m map[string]*fullstats.RequestStats
	var err error
	if scope == "session" {
		m, err = tracker.GetSessionRequests()
	} else {
		m, err = tracker.GetAllRequests()
	}
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	out := make([]requestRowSortable, 0, len(m))
	for k, st := range m {
		if st == nil {
			continue
		}
		cp := *st
		if st.SourceCount != nil {
			cp.SourceCount = make(map[string]uint64, len(st.SourceCount))
			for sk, sv := range st.SourceCount {
				cp.SourceCount[sk] = sv
			}
		}
		out = append(out, requestRowSortable{key: k, st: &cp})
	}
	return out, nil
}

// filterRequestRows keeps rows whose key, domain, record type, or source label matches q (case-insensitive substring).
func filterRequestRows(rows []requestRowSortable, needle string) []requestRowSortable {
	if needle == "" {
		return rows
	}
	n := strings.ToLower(needle)
	out := make([]requestRowSortable, 0, len(rows))
	for _, row := range rows {
		domain, rtype := splitDomainType(row.key)
		if strings.Contains(strings.ToLower(row.key), n) ||
			strings.Contains(strings.ToLower(domain), n) ||
			strings.Contains(strings.ToLower(rtype), n) {
			out = append(out, row)
			continue
		}
		if row.st != nil && row.st.SourceCount != nil {
			for sk := range row.st.SourceCount {
				if strings.Contains(strings.ToLower(sk), n) {
					out = append(out, row)
					break
				}
			}
		}
	}
	return out
}

// filterRequesterRows keeps rows whose IP or any counted record type contains q (case-insensitive substring).
func filterRequesterRows(rows []requesterRowSortable, needle string) []requesterRowSortable {
	if needle == "" {
		return rows
	}
	n := strings.ToLower(needle)
	out := make([]requesterRowSortable, 0, len(rows))
rows:
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.ip), n) {
			out = append(out, row)
			continue
		}
		if row.st != nil && row.st.TypeCount != nil {
			for rk := range row.st.TypeCount {
				if strings.Contains(strings.ToLower(rk), n) {
					out = append(out, row)
					continue rows
				}
			}
		}
		if row.st != nil && row.st.SourceCount != nil {
			for sk := range row.st.SourceCount {
				if strings.Contains(strings.ToLower(sk), n) {
					out = append(out, row)
					continue rows
				}
			}
		}
	}
	return out
}

func sortRequestRows(rows []requestRowSortable, sortKey string) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch sortKey {
		case "count_asc":
			if a.st.Count != b.st.Count {
				return a.st.Count < b.st.Count
			}
			return a.key < b.key
		case "key_asc":
			return a.key < b.key
		case "last_seen_desc":
			if !a.st.LastSeen.Equal(b.st.LastSeen) {
				return a.st.LastSeen.After(b.st.LastSeen)
			}
			return a.key < b.key
		case "last_seen_asc":
			if !a.st.LastSeen.Equal(b.st.LastSeen) {
				return a.st.LastSeen.Before(b.st.LastSeen)
			}
			return a.key < b.key
		case "first_seen_desc":
			if !a.st.FirstSeen.Equal(b.st.FirstSeen) {
				return a.st.FirstSeen.After(b.st.FirstSeen)
			}
			return a.key < b.key
		case "first_seen_asc":
			if !a.st.FirstSeen.Equal(b.st.FirstSeen) {
				return a.st.FirstSeen.Before(b.st.FirstSeen)
			}
			return a.key < b.key
		default: // count_desc
			if a.st.Count != b.st.Count {
				return a.st.Count > b.st.Count
			}
			return a.key < b.key
		}
	})
}

type requesterRowSortable struct {
	ip    string
	st    *fullstats.RequesterStats
	total uint64
}

func requesterTotal(st *fullstats.RequesterStats) uint64 {
	var t uint64
	if st == nil || st.TypeCount == nil {
		return 0
	}
	for _, c := range st.TypeCount {
		t += c
	}
	return t
}

func collectRequesterRows(tracker *fullstats.Tracker, scope string) ([]requesterRowSortable, error) {
	var m map[string]*fullstats.RequesterStats
	var err error
	if scope == "session" {
		m, err = tracker.GetSessionRequesters()
	} else {
		m, err = tracker.GetAllRequesters()
	}
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	out := make([]requesterRowSortable, 0, len(m))
	for ip, st := range m {
		if st == nil {
			continue
		}
		cp := *st
		if cp.TypeCount != nil {
			tc := make(map[string]uint64, len(cp.TypeCount))
			for k, v := range cp.TypeCount {
				tc[k] = v
			}
			cp.TypeCount = tc
		}
		if cp.SourceCount != nil {
			sc := make(map[string]uint64, len(cp.SourceCount))
			for k, v := range cp.SourceCount {
				sc[k] = v
			}
			cp.SourceCount = sc
		}
		tot := requesterTotal(&cp)
		out = append(out, requesterRowSortable{ip: ip, st: &cp, total: tot})
	}
	return out, nil
}

func sortRequesterRows(rows []requesterRowSortable, sortKey string) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch sortKey {
		case "total_asc":
			if a.total != b.total {
				return a.total < b.total
			}
			return a.ip < b.ip
		case "ip_asc":
			return a.ip < b.ip
		case "first_seen_desc":
			if !a.st.FirstSeen.Equal(b.st.FirstSeen) {
				return a.st.FirstSeen.After(b.st.FirstSeen)
			}
			return a.ip < b.ip
		case "first_seen_asc":
			if !a.st.FirstSeen.Equal(b.st.FirstSeen) {
				return a.st.FirstSeen.Before(b.st.FirstSeen)
			}
			return a.ip < b.ip
		default: // total_desc
			if a.total != b.total {
				return a.total > b.total
			}
			return a.ip < b.ip
		}
	})
}

func paginateSlice[T any](rows []T, page, perPage int) (outPage, totalPages int, slice []T) {
	n := len(rows)
	totalPages = 1
	if n > 0 {
		totalPages = (n + perPage - 1) / perPage
	}
	outPage = page
	if outPage < 1 {
		outPage = 1
	}
	if outPage > totalPages {
		outPage = totalPages
	}
	if n == 0 {
		return outPage, totalPages, nil
	}
	offset := (outPage - 1) * perPage
	end := offset + perPage
	if end > n {
		end = n
	}
	return outPage, totalPages, rows[offset:end]
}

type fullStatsRequestRowJSON struct {
	Key         string            `json:"key"`
	Domain      string            `json:"domain"`
	RecordType  string            `json:"record_type"`
	Count       uint64            `json:"count"`
	SourceCount map[string]uint64 `json:"source_count,omitempty"`
	FirstSeen   string            `json:"first_seen"`
	LastSeen    string            `json:"last_seen"`
}

type fullStatsRequesterRowJSON struct {
	IP            string            `json:"ip"`
	TotalRequests uint64            `json:"total_requests"`
	FirstSeen     string            `json:"first_seen"`
	ByType        map[string]uint64 `json:"by_type"`
	BySource      map[string]uint64 `json:"by_source,omitempty"`
}

type fullStatsBrowseResponseJSON struct {
	Enabled    bool   `json:"enabled"`
	Message    string `json:"message,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Table      string `json:"table,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Q          string `json:"q,omitempty"`
	Page       int    `json:"page,omitempty"`
	PerPage    int    `json:"per_page,omitempty"`
	TotalRows  int    `json:"total_rows,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	Rows       any    `json:"rows,omitempty"`
}
