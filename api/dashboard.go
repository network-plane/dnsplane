// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package api

import (
	"sort"
	"strings"
	"time"

	"dnsplane/fullstats"
)

const dashboardFullstatsTopN = 10

func topRequestersForDashboard(all map[string]*fullstats.RequesterStats, limit int) []struct {
	IP        string
	Total     uint64
	FirstSeen string
} {
	type row struct {
		ip    string
		total uint64
		first time.Time
	}
	var rows []row
	for ip, st := range all {
		var total uint64
		for _, c := range st.TypeCount {
			total += c
		}
		rows = append(rows, row{ip: ip, total: total, first: st.FirstSeen})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].total > rows[j].total })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]struct {
		IP        string
		Total     uint64
		FirstSeen string
	}, len(rows))
	for i, r := range rows {
		out[i].IP = r.ip
		out[i].Total = r.total
		out[i].FirstSeen = r.first.Format(time.RFC3339)
	}
	return out
}

func topDomainsForDashboard(all map[string]*fullstats.RequestStats, limit int) []struct {
	Domain              string
	RecordType          string
	Count               uint64
	FirstSeen, LastSeen string
} {
	type row struct {
		domain, rtype string
		count         uint64
		first, last   time.Time
	}
	var rows []row
	for key, st := range all {
		domain, rtype := splitDomainType(key)
		rows = append(rows, row{domain: domain, rtype: rtype, count: st.Count, first: st.FirstSeen, last: st.LastSeen})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].count > rows[j].count })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]struct {
		Domain              string
		RecordType          string
		Count               uint64
		FirstSeen, LastSeen string
	}, len(rows))
	for i, r := range rows {
		out[i].Domain = r.domain
		out[i].RecordType = r.rtype
		out[i].Count = r.count
		out[i].FirstSeen = r.first.Format(time.RFC3339)
		out[i].LastSeen = r.last.Format(time.RFC3339)
	}
	return out
}

// splitDomainType splits a fullstats key "domain:type" into domain and record type.
// It splits on the last ":" so that domain names containing ":" still parse correctly.
func splitDomainType(key string) (domain, recordType string) {
	if key == "" {
		return "", ""
	}
	if i := strings.LastIndex(key, ":"); i >= 0 {
		return key[:i], key[i+1:]
	}
	return key, ""
}
