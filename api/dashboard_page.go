// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"
	"time"

	"dnsplane/cluster"
	"dnsplane/data"
)

func dashboardDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	dnsData := data.GetInstance()
	stats := dnsData.GetStats()
	perf := data.GetResolverPerfReport()

	tq := stats.TotalQueries
	tch := stats.TotalCacheHits
	tb := stats.TotalBlocks
	var cacheHitRatio any
	var blockRate any
	if tq > 0 {
		cacheHitRatio = float64(tch) / float64(tq)
		blockRate = float64(tb) / float64(tq)
	}

	activeUpstreams := 0
	for _, s := range dnsData.GetServers() {
		if s.Active {
			activeUpstreams++
		}
	}
	st, healthChecksOn := dnsData.UpstreamHealthStatuses()
	upstreamHealth := map[string]any{
		"checks_enabled":  healthChecksOn,
		"active":          activeUpstreams,
		"configured_rows": len(st),
		"healthy":         0,
		"unhealthy":       0,
	}
	if len(st) > 0 {
		bad := 0
		for i := range st {
			if st[i].Unhealthy {
				bad++
			}
		}
		upstreamHealth["healthy"] = len(st) - bad
		upstreamHealth["unhealthy"] = bad
	}

	summary := map[string]any{
		"server_start":    stats.ServerStartTime.UTC().Format(time.RFC3339),
		"uptime_seconds":  int64(time.Since(stats.ServerStartTime).Seconds()),
		"cache_hit_ratio": cacheHitRatio,
		"block_rate":      blockRate,
		"total_blocks":    tb,
		"upstream":        upstreamHealth,
	}

	payload := map[string]any{
		"counters": map[string]any{
			"total_queries":           stats.TotalQueries,
			"total_cache_hits":        stats.TotalCacheHits,
			"total_blocks":            stats.TotalBlocks,
			"total_queries_forwarded": stats.TotalQueriesForwarded,
			"total_queries_answered":  stats.TotalQueriesAnswered,
		},
		"perf": map[string]any{
			"total":            perf.AResolve.Total,
			"outcome_local":    perf.AResolve.OutcomeLocal,
			"outcome_cache":    perf.AResolve.OutcomeCache,
			"outcome_upstream": perf.AResolve.OutcomeUpstream,
			"outcome_none":     perf.AResolve.OutcomeNone,
			"avg_total_ms":     perf.AResolve.AvgTotalMs,
			"max_total_ms":     perf.AResolve.MaxTotalMs,
		},
		"summary": summary,
		"series":  data.GetDashboardSeries(),
		"log":     data.GetDashboardLogNewestFirst(200),
	}
	if tot, sess, ok := getFullStatsCounts(); ok {
		payload["fullstats"] = map[string]any{
			"domains_total":      tot.DomainsCount,
			"requesters_total":   tot.RequestersCount,
			"domains_session":    sess.DomainsCount,
			"requesters_session": sess.RequestersCount,
		}
	}
	if mgr := cluster.GlobalManager(); mgr != nil {
		snap := mgr.StatusSnapshot()
		payload["cluster"] = snap
	}
	payload["build"] = BuildInfo()
	writeJSON(w, http.StatusOK, payload)
}

func dashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}

// dashboardHTML is a self-contained dashboard (dark theme aligned with /stats/page; Chart.js from CDN).
// Left nav embeds Stats/Perf/Metrics/Version in the right pane; Ctrl/Cmd-click opens the real URL in a new tab.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dnsplane — Dashboard</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js" crossorigin="anonymous"></script>
  <style>
    :root {
      --bg: #0d1117;
      --surface: #161b22;
      --surface-hover: #21262d;
      --border: #30363d;
      --text: #e6edf3;
      --muted: #8b949e;
      --accent: #58a6ff;
      --accent-soft: rgba(88, 166, 255, 0.12);
      --success: #3fb950;
      --warning: #d29922;
      --danger: #f85149;
      --sidebar-w: 220px;
      --radius: 8px;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
    }
    .app {
      display: flex;
      min-height: 100vh;
    }
    aside {
      width: var(--sidebar-w);
      background: var(--surface);
      border-right: 1px solid var(--border);
      padding: 1.25rem 0;
      flex-shrink: 0;
    }
    .brand {
      padding: 0 1.25rem 1.25rem;
      font-weight: 700;
      font-size: 1.1rem;
      color: var(--accent);
      border-bottom: 1px solid var(--border);
      margin-bottom: 1rem;
    }
    nav a {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.65rem 1.25rem;
      color: var(--muted);
      text-decoration: none;
      font-size: 0.9rem;
    }
    nav a:hover { background: var(--surface-hover); color: var(--text); }
    nav a.active {
      background: var(--accent-soft);
      color: var(--accent);
      font-weight: 600;
      border-right: 3px solid var(--accent);
    }
    main.main-shell {
      flex: 1;
      display: flex;
      flex-direction: column;
      min-height: 100vh;
      min-width: 0;
      padding: 1.5rem 1.75rem;
      overflow: hidden;
    }
    #view-dashboard {
      flex: 1;
      display: flex;
      flex-direction: column;
      min-height: 0;
      overflow: auto;
    }
    #view-dashboard.hidden { display: none; }
    iframe.view-embed {
      flex: 1;
      width: 100%;
      min-height: 0;
      border: none;
      background: var(--bg);
      border-radius: var(--radius);
    }
    iframe.view-embed.hidden { display: none; }
    h1 {
      font-size: 1.35rem;
      font-weight: 600;
      margin: 0 0 1.25rem 0;
    }
    .metric-row {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 1rem;
      margin-bottom: 1.25rem;
    }
    @media (max-width: 1100px) {
      .metric-row { grid-template-columns: repeat(2, 1fr); }
    }
    @media (max-width: 700px) {
      .app { flex-direction: column; }
      aside { width: 100%; border-right: none; border-bottom: 1px solid var(--border); }
      .metric-row { grid-template-columns: 1fr; }
    }
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 1rem 1.15rem;
    }
    .card h3 {
      margin: 0 0 0.35rem 0;
      font-size: 0.75rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted);
    }
    .card .value {
      font-size: 1.75rem;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--text);
    }
    .card .sub {
      font-size: 0.8rem;
      color: var(--muted);
      margin-top: 0.35rem;
    }
    .dash-split-row {
      display: flex;
      gap: 1rem;
      align-items: stretch;
      margin-bottom: 1.25rem;
      flex-wrap: wrap;
    }
    .dash-narrow {
      flex: 0 0 200px;
      max-width: 240px;
      min-width: 180px;
    }
    .dash-cluster {
      flex: 1 1 280px;
      min-width: 0;
      margin-bottom: 0;
    }
    .charts-row {
      display: grid;
      grid-template-columns: 1fr 380px;
      gap: 1.25rem;
      align-items: start;
    }
    @media (max-width: 1200px) {
      .charts-row { grid-template-columns: 1fr; }
    }
    .charts-stack {
      display: flex;
      flex-direction: column;
      gap: 1rem;
    }
    .chart-card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 1rem 1.15rem;
    }
    .chart-card h2 {
      margin: 0 0 0.75rem 0;
      font-size: 0.95rem;
      font-weight: 600;
    }
    .chart-wrap {
      position: relative;
      height: 220px;
    }
    .activity-panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      display: flex;
      flex-direction: column;
      max-height: calc(100vh - 8rem);
      min-height: 480px;
    }
    .activity-head {
      padding: 1rem 1.15rem;
      border-bottom: 1px solid var(--border);
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .activity-head h2 {
      margin: 0;
      font-size: 1rem;
      font-weight: 600;
    }
    .activity-head .muted { font-size: 0.8rem; color: var(--muted); }
    .activity-body {
      overflow-y: auto;
      flex: 1;
      padding: 0.5rem 0;
    }
    .log-item {
      padding: 0.65rem 1.15rem;
      border-bottom: 1px solid var(--border);
      font-size: 0.82rem;
    }
    .log-item:last-child { border-bottom: none; }
    .log-item .top {
      display: flex;
      align-items: flex-start;
      gap: 0.5rem;
    }
    .dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      margin-top: 0.35rem;
      flex-shrink: 0;
    }
    .dot.local { background: var(--success); }
    .dot.cache { background: var(--accent); }
    .dot.upstream { background: var(--warning); }
    .dot.blocked { background: var(--danger); }
    .dot.none { background: var(--muted); }
    .log-query {
      font-weight: 600;
      color: var(--text);
    }
    .log-meta {
      color: var(--muted);
      font-size: 0.78rem;
      margin-top: 0.2rem;
    }
    .log-record {
      margin-top: 0.35rem;
      color: var(--text);
      word-break: break-all;
      line-height: 1.35;
    }
    .log-time {
      float: right;
      color: var(--muted);
      font-size: 0.75rem;
    }
    .err { color: var(--danger); padding: 1rem; }
    .muted-link { color: var(--muted); font-size: 0.85rem; }
    .muted-link a { color: var(--accent); }
    code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.85em;
      background: var(--surface-hover);
      padding: 0.15rem 0.4rem;
      border-radius: 4px;
    }
    #view-fullstats {
      flex: 1;
      display: flex;
      flex-direction: column;
      min-height: 0;
      overflow: auto;
    }
    #view-fullstats.hidden { display: none; }
    .fs-toolbar {
      display: flex;
      flex-wrap: wrap;
      gap: 0.75rem 1.25rem;
      align-items: center;
      margin-bottom: 1rem;
    }
    .fs-toolbar label {
      display: flex;
      align-items: center;
      gap: 0.4rem;
      font-size: 0.8rem;
      color: var(--muted);
    }
    .fs-toolbar select {
      background: var(--surface);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 0.35rem 0.5rem;
      font-size: 0.85rem;
    }
    .fs-toolbar input[type="search"] {
      min-width: 12rem;
      max-width: 28rem;
      flex: 1 1 12rem;
      background: var(--surface);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 0.35rem 0.65rem;
      font-size: 0.85rem;
    }
    .fs-toolbar input[type="search"]::placeholder { color: var(--muted); }
    .fs-btn {
      background: var(--surface-hover);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 0.4rem 0.85rem;
      font-size: 0.85rem;
      cursor: pointer;
    }
    .fs-btn:hover:not(:disabled) { background: #30363d; }
    .fs-btn:disabled { opacity: 0.45; cursor: not-allowed; }
    .fs-table-wrap {
      overflow-x: auto;
      border: 1px solid var(--border);
      border-radius: var(--radius);
      background: var(--surface);
    }
    .fs-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.84rem;
    }
    .fs-table th, .fs-table td {
      padding: 0.55rem 0.75rem;
      text-align: left;
      border-bottom: 1px solid var(--border);
      vertical-align: top;
    }
    .fs-table th {
      color: var(--muted);
      font-weight: 600;
      font-size: 0.76rem;
      text-transform: uppercase;
      letter-spacing: 0.03em;
      white-space: nowrap;
    }
    .fs-table tbody tr:hover { background: var(--surface-hover); }
    .fs-table td.num { font-variant-numeric: tabular-nums; text-align: right; }
    .fs-table th.num { text-align: right; }
    .fs-by-type { font-size: 0.78rem; color: var(--muted); max-width: 28rem; word-break: break-word; }
    .fs-pagination {
      display: flex;
      flex-wrap: wrap;
      gap: 0.75rem 1rem;
      align-items: center;
      margin-top: 1rem;
      font-size: 0.85rem;
    }
  </style>
</head>
<body>
  <div class="app">
    <aside>
      <div class="brand">dnsplane</div>
      <nav>
        <a class="active" href="/stats/dashboard" data-view="dashboard">Dashboard</a>
        <a href="/stats/dashboard" data-view="fullstats">Stored stats</a>
        <a href="/stats/page" data-embed="1">Stats</a>
        <a href="/stats/perf/page" data-embed="1">Perf</a>
        <a href="/metrics" data-embed="1">Metrics</a>
        <a href="/version" data-embed="1">Version</a>
      </nav>
    </aside>
    <main class="main-shell">
      <div id="view-dashboard">
        <h1>Dashboard</h1>
        <p class="muted-link" style="margin:-0.5rem 0 1rem 0">Live data · refreshes every 2s while this view is open · <a href="/stats/dashboard/data">JSON</a></p>
        <div id="err" class="err"></div>
        <div class="metric-row">
          <div class="card"><h3>Total queries</h3><div class="value" id="m-queries">—</div><div class="sub">since start</div></div>
          <div class="card"><h3>Cache hits</h3><div class="value" id="m-cache">—</div><div class="sub">dnscache</div></div>
          <div class="card"><h3>Avg resolve (fast path)</h3><div class="value" id="m-avg">—</div><div class="sub">ms · A/AAAA perf</div></div>
          <div class="card"><h3>Upstream wins</h3><div class="value" id="m-up">—</div><div class="sub">outcome_upstream</div></div>
        </div>
        <div class="dash-split-row">
          <div class="card dash-narrow">
            <h3>Cache hit ratio</h3>
            <div class="value" id="m-cache-pct">—</div>
            <div class="sub">hits ÷ queries</div>
          </div>
          <div class="chart-card dash-cluster" id="cluster-wrap" style="display:none">
            <h2>Cluster</h2>
            <div id="cluster-root" style="font-size:0.85rem"></div>
          </div>
        </div>
        <div class="metric-row">
          <div class="card"><h3>Block rate</h3><div class="value" id="m-block-rate">—</div><div class="sub">blocks ÷ queries</div></div>
          <div class="card"><h3>Total blocks</h3><div class="value" id="m-blocks">—</div><div class="sub">adblock</div></div>
          <div class="card"><h3>Uptime</h3><div class="value" id="m-uptime">—</div><div class="sub">since start</div></div>
          <div class="card"><h3>Upstreams</h3><div class="value" id="m-up-health">—</div><div class="sub">healthy / configured</div></div>
        </div>
        <div class="metric-row">
          <div class="card"><h3>Answered</h3><div class="value" id="m-answered">—</div><div class="sub">queries answered</div></div>
          <div class="card"><h3>Forwarded</h3><div class="value" id="m-forwarded">—</div><div class="sub">upstream forwards</div></div>
          <div class="card"><h3>A/AAAA samples</h3><div class="value" id="m-perf-total">—</div><div class="sub">perf fast-path count</div></div>
          <div class="card"><h3>Version</h3><div class="value" id="m-version" style="font-size:1.1rem">—</div><div class="sub">build</div></div>
        </div>
        <div class="metric-row">
          <div class="card"><h3>Local</h3><div class="value" id="m-oc-local">—</div><div class="sub">A/AAAA outcome</div></div>
          <div class="card"><h3>Cache</h3><div class="value" id="m-oc-cache">—</div><div class="sub">A/AAAA outcome</div></div>
          <div class="card"><h3>Upstream</h3><div class="value" id="m-oc-up">—</div><div class="sub">A/AAAA outcome</div></div>
          <div class="card"><h3>None</h3><div class="value" id="m-oc-none">—</div><div class="sub">no answer</div></div>
        </div>
        <div class="metric-row" id="fullstats-kpi-row" style="display:none">
          <div class="card"><h3>Stored domains</h3><div class="value" id="m-fs-dom">—</div><div class="sub">full_stats · total</div></div>
          <div class="card"><h3>Stored requesters</h3><div class="value" id="m-fs-req">—</div><div class="sub">full_stats · total</div></div>
          <div class="card"><h3>Domains (session)</h3><div class="value" id="m-fs-dom-s">—</div><div class="sub">since process start</div></div>
          <div class="card"><h3>Requesters (session)</h3><div class="value" id="m-fs-req-s">—</div><div class="sub">since process start</div></div>
        </div>
        <div class="charts-row">
          <div class="charts-stack">
            <div class="chart-card">
              <h2>Replies per minute</h2>
              <div class="chart-wrap"><canvas id="chartReplies"></canvas></div>
            </div>
            <div class="chart-card">
              <h2>Average resolution time</h2>
              <div class="chart-wrap"><canvas id="chartLatency"></canvas></div>
            </div>
          </div>
          <div class="activity-panel">
            <div class="activity-head">
              <h2>Resolutions</h2>
              <span class="muted">newest first</span>
            </div>
            <div class="activity-body" id="log-root"></div>
          </div>
        </div>
      </div>
      <div id="view-fullstats" class="hidden">
        <h1>Stored statistics</h1>
        <p class="muted-link" style="margin:-0.5rem 0 1rem 0">Full_stats database (<code>stats.db</code>) and session counters · <a id="fs-json-link" href="/stats/dashboard/fullstats/data" target="_blank" rel="noopener">JSON API</a></p>
        <div id="fs-msg" class="err" style="display:none"></div>
        <div class="fs-toolbar">
          <label>Scope <select id="fs-scope" aria-label="Scope">
            <option value="total">Total (persisted)</option>
            <option value="session">Session (since start)</option>
          </select></label>
          <label>Data <select id="fs-table" aria-label="Data set">
            <option value="requests">Domain : type</option>
            <option value="requesters">Requesters (by IP)</option>
          </select></label>
          <label>Sort <select id="fs-sort" aria-label="Sort"></select></label>
          <label>Per page <select id="fs-per" aria-label="Rows per page">
            <option value="25">25</option>
            <option value="50">50</option>
            <option value="100">100</option>
          </select></label>
          <label style="flex:1 1 14rem;min-width:10rem">Search <input type="search" id="fs-q" placeholder="Name, type, key, IP, or source (local, cache, …)" autocomplete="off" spellcheck="false" aria-label="Filter rows"></label>
          <button type="button" class="fs-btn" id="fs-refresh">Refresh</button>
        </div>
        <div class="fs-table-wrap">
          <table class="fs-table" id="fs-table-el">
            <thead id="fs-thead"></thead>
            <tbody id="fs-tbody"></tbody>
          </table>
        </div>
        <div class="fs-pagination">
          <button type="button" class="fs-btn" id="fs-prev">Previous</button>
          <span id="fs-page-info" class="muted-link"></span>
          <button type="button" class="fs-btn" id="fs-next">Next</button>
          <span id="fs-total" class="muted-link"></span>
        </div>
      </div>
      <iframe id="view-embed" class="view-embed hidden" title="Embedded view" sandbox="allow-scripts allow-same-origin"></iframe>
    </main>
  </div>
  <script>
    const fmtMs = (x) => (x == null || isNaN(x)) ? '—' : (Number(x) < 10 ? x.toFixed(3) : x.toFixed(2));
    function dotClass(o) {
      const m = { local: 'local', cache: 'cache', upstream: 'upstream', blocked: 'blocked', none: 'none' };
      return m[o] || 'none';
    }
    function relTime(iso) {
      try {
        const t = new Date(iso).getTime();
        const s = Math.floor((Date.now() - t) / 1000);
        if (s < 60) return s + 's ago';
        if (s < 3600) return Math.floor(s/60) + 'm ago';
        return Math.floor(s/3600) + 'h ago';
      } catch(e) { return ''; }
    }
    function esc(s) {
      if (s == null) return '';
      return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }
    function fmtPctRatio(x) {
      if (x == null || isNaN(x)) return '—';
      return (Number(x) * 100).toFixed(1) + '%';
    }
    function fmtUptimeSec(sec) {
      if (sec == null || isNaN(sec) || sec < 0) return '—';
      var s = Math.floor(Number(sec));
      var d = Math.floor(s / 86400);
      s -= d * 86400;
      var h = Math.floor(s / 3600);
      s -= h * 3600;
      var m = Math.floor(s / 60);
      if (d > 0) return d + 'd ' + h + 'h';
      if (h > 0) return h + 'h ' + m + 'm';
      return m + 'm';
    }
    let chartReplies, chartLatency;
    let dashboardTimer = null;
    function chartCommon() {
      const grid = '#30363d';
      const tick = '#8b949e';
      return {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: {
            grid: { color: grid },
            ticks: { color: tick, maxRotation: 45, font: { size: 10 } }
          },
          y: {
            beginAtZero: true,
            grid: { color: grid },
            ticks: { color: tick }
          }
        },
        plugins: { legend: { display: false } }
      };
    }
    function initCharts() {
      const common = chartCommon();
      chartReplies = new Chart(document.getElementById('chartReplies'), {
        type: 'line',
        data: { labels: [], datasets: [{ label: 'Replies', data: [], borderColor: '#3fb950', backgroundColor: 'rgba(63,185,80,0.12)', fill: true, tension: 0.25 }] },
        options: common
      });
      chartLatency = new Chart(document.getElementById('chartLatency'), {
        type: 'line',
        data: { labels: [], datasets: [{ label: 'Avg ms', data: [], borderColor: '#58a6ff', backgroundColor: 'rgba(88,166,255,0.1)', fill: true, tension: 0.25 }] },
        options: common
      });
    }
    function shortLabel(iso) {
      if (!iso) return '';
      const d = new Date(iso);
      const hh = d.getHours().toString().padStart(2,'0');
      const mm = d.getMinutes().toString().padStart(2,'0');
      return hh + ':' + mm;
    }
    function stopDashboardRefresh() {
      if (dashboardTimer) {
        clearInterval(dashboardTimer);
        dashboardTimer = null;
      }
    }
    function startDashboardRefresh() {
      stopDashboardRefresh();
      load();
      dashboardTimer = setInterval(load, 2000);
    }
    function setActiveNav(el) {
      document.querySelectorAll('aside nav a').forEach(function(a) {
        a.classList.toggle('active', a === el);
      });
    }
    var fsState = { scope: 'total', table: 'requests', sort: 'count_desc', page: 1, perPage: 25, search: '' };
    function fsSortOptions(table) {
      if (table === 'requesters') {
        return [
          ['total_desc', 'Total (high → low)'],
          ['total_asc', 'Total (low → high)'],
          ['ip_asc', 'IP (A–Z)'],
          ['first_seen_desc', 'First seen (newest)'],
          ['first_seen_asc', 'First seen (oldest)']
        ];
      }
      return [
        ['count_desc', 'Count (high → low)'],
        ['count_asc', 'Count (low → high)'],
        ['key_asc', 'Key (A–Z)'],
        ['last_seen_desc', 'Last seen (newest)'],
        ['last_seen_asc', 'Last seen (oldest)'],
        ['first_seen_desc', 'First seen (newest)'],
        ['first_seen_asc', 'First seen (oldest)']
      ];
    }
    function fsSyncSortSelect() {
      var sel = document.getElementById('fs-sort');
      var opts = fsSortOptions(fsState.table);
      var allowed = {};
      for (var i = 0; i < opts.length; i++) allowed[opts[i][0]] = true;
      if (!allowed[fsState.sort]) fsState.sort = opts[0][0];
      sel.innerHTML = '';
      for (var j = 0; j < opts.length; j++) {
        var o = document.createElement('option');
        o.value = opts[j][0];
        o.textContent = opts[j][1];
        if (opts[j][0] === fsState.sort) o.selected = true;
        sel.appendChild(o);
      }
    }
    function fsBuildQuery() {
      var p = new URLSearchParams();
      p.set('scope', fsState.scope);
      p.set('table', fsState.table);
      p.set('sort', fsState.sort);
      p.set('page', String(fsState.page));
      p.set('per_page', String(fsState.perPage));
      if (fsState.search) p.set('q', fsState.search);
      return p.toString();
    }
    function fsFormatByType(by) {
      if (!by || typeof by !== 'object') return '—';
      var keys = Object.keys(by).sort();
      if (!keys.length) return '—';
      return keys.map(function(k) { return k + ': ' + by[k]; }).join(', ');
    }
    function fsReadControls() {
      fsState.scope = document.getElementById('fs-scope').value;
      fsState.table = document.getElementById('fs-table').value;
      fsState.sort = document.getElementById('fs-sort').value;
      fsState.perPage = parseInt(document.getElementById('fs-per').value, 10) || 25;
      fsState.search = (document.getElementById('fs-q').value || '').trim();
    }
    async function loadFullStats() {
      var msg = document.getElementById('fs-msg');
      msg.style.display = 'none';
      msg.textContent = '';
      var qs = fsBuildQuery();
      document.getElementById('fs-json-link').href = '/stats/dashboard/fullstats/data?' + qs;
      try {
        var r = await fetch('/stats/dashboard/fullstats/data?' + qs);
        if (!r.ok) throw new Error('HTTP ' + r.status);
        var j = await r.json();
        if (!j.enabled) {
          msg.style.display = 'block';
          msg.textContent = j.message || 'Full statistics are not enabled.';
          document.getElementById('fs-tbody').innerHTML = '';
          document.getElementById('fs-thead').innerHTML = '';
          document.getElementById('fs-page-info').textContent = '';
          document.getElementById('fs-total').textContent = '';
          document.getElementById('fs-prev').disabled = true;
          document.getElementById('fs-next').disabled = true;
          return;
        }
        fsState.page = j.page;
        fsState.perPage = j.per_page;
        document.getElementById('fs-scope').value = j.scope;
        document.getElementById('fs-table').value = j.table;
        document.getElementById('fs-per').value = String(j.per_page);
        fsState.scope = j.scope;
        fsState.table = j.table;
        fsState.sort = j.sort;
        if (typeof j.q === 'string') {
          fsState.search = j.q;
          document.getElementById('fs-q').value = j.q;
        }
        fsSyncSortSelect();
        document.getElementById('fs-sort').value = j.sort;
        var thead = document.getElementById('fs-thead');
        var tbody = document.getElementById('fs-tbody');
        if (j.table === 'requesters') {
          thead.innerHTML = '<tr><th>IP</th><th class="num">Total</th><th>First seen</th><th>By record type</th><th>Reply from</th></tr>';
          var rows = j.rows || [];
          var h = '';
          for (var i = 0; i < rows.length; i++) {
            var row = rows[i];
            h += '<tr><td style="font-family:ui-monospace,monospace">' + esc(row.ip) + '</td>';
            h += '<td class="num">' + esc(String(row.total_requests)) + '</td>';
            h += '<td>' + esc(row.first_seen || '—') + '</td>';
            h += '<td class="fs-by-type">' + esc(fsFormatByType(row.by_type)) + '</td>';
            h += '<td class="fs-by-type">' + esc(fsFormatByType(row.by_source)) + '</td></tr>';
          }
          if (!rows.length) h = '<tr><td colspan="5" style="color:var(--muted)">No requester rows for this scope.</td></tr>';
          tbody.innerHTML = h;
        } else {
          thead.innerHTML = '<tr><th>Domain</th><th>Type</th><th class="num">Count</th><th>Reply from</th><th>First seen</th><th>Last seen</th><th>Key</th></tr>';
          var rows2 = j.rows || [];
          var h2 = '';
          for (var k = 0; k < rows2.length; k++) {
            var r2 = rows2[k];
            h2 += '<tr><td>' + esc(r2.domain) + '</td><td>' + esc(r2.record_type) + '</td>';
            h2 += '<td class="num">' + esc(String(r2.count)) + '</td>';
            h2 += '<td class="fs-by-type">' + esc(fsFormatByType(r2.source_count)) + '</td>';
            h2 += '<td>' + esc(r2.first_seen || '—') + '</td><td>' + esc(r2.last_seen || '—') + '</td>';
            h2 += '<td style="font-family:ui-monospace,monospace;font-size:0.78rem">' + esc(r2.key) + '</td></tr>';
          }
          if (!rows2.length) h2 = '<tr><td colspan="7" style="color:var(--muted)">No domain:type rows for this scope.</td></tr>';
          tbody.innerHTML = h2;
        }
        var tp = j.total_pages || 1;
        var tr = j.total_rows != null ? j.total_rows : 0;
        document.getElementById('fs-page-info').textContent = 'Page ' + j.page + ' of ' + tp;
        document.getElementById('fs-total').textContent = tr + ' row' + (tr === 1 ? '' : 's') + ' total';
        document.getElementById('fs-prev').disabled = j.page <= 1;
        document.getElementById('fs-next').disabled = j.page >= tp;
      } catch (e) {
        msg.style.display = 'block';
        msg.textContent = 'Failed to load: ' + e.message;
      }
    }
    function wireFullStats() {
      fsSyncSortSelect();
      document.getElementById('fs-scope').addEventListener('change', function() { fsReadControls(); fsState.page = 1; loadFullStats(); });
      document.getElementById('fs-table').addEventListener('change', function() { fsReadControls(); fsState.page = 1; fsSyncSortSelect(); fsState.sort = document.getElementById('fs-sort').value; loadFullStats(); });
      document.getElementById('fs-sort').addEventListener('change', function() { fsReadControls(); fsState.page = 1; loadFullStats(); });
      document.getElementById('fs-per').addEventListener('change', function() { fsReadControls(); fsState.page = 1; loadFullStats(); });
      document.getElementById('fs-refresh').addEventListener('click', function() { fsReadControls(); loadFullStats(); });
      document.getElementById('fs-q').addEventListener('keydown', function(e) {
        if (e.key === 'Enter') { e.preventDefault(); fsReadControls(); fsState.page = 1; loadFullStats(); }
      });
      document.getElementById('fs-prev').addEventListener('click', function() { fsReadControls(); if (fsState.page > 1) { fsState.page--; loadFullStats(); } });
      document.getElementById('fs-next').addEventListener('click', function() { fsReadControls(); fsState.page++; loadFullStats(); });
    }
    function showDashboard(anchor) {
      if (anchor) setActiveNav(anchor);
      document.getElementById('view-dashboard').classList.remove('hidden');
      document.getElementById('view-fullstats').classList.add('hidden');
      const iframe = document.getElementById('view-embed');
      iframe.classList.add('hidden');
      iframe.src = 'about:blank';
      startDashboardRefresh();
    }
    function showFullStats(anchor) {
      if (anchor) setActiveNav(anchor);
      stopDashboardRefresh();
      document.getElementById('view-dashboard').classList.add('hidden');
      document.getElementById('view-fullstats').classList.remove('hidden');
      const iframe = document.getElementById('view-embed');
      iframe.classList.add('hidden');
      iframe.src = 'about:blank';
      fsReadControls();
      fsSyncSortSelect();
      loadFullStats();
    }
    function showEmbed(url, anchor) {
      if (anchor) setActiveNav(anchor);
      stopDashboardRefresh();
      document.getElementById('view-dashboard').classList.add('hidden');
      document.getElementById('view-fullstats').classList.add('hidden');
      const iframe = document.getElementById('view-embed');
      iframe.classList.remove('hidden');
      iframe.src = url;
    }
    document.querySelectorAll('aside nav a').forEach(function(a) {
      a.addEventListener('click', function(e) {
        if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey || e.button !== 0) return;
        if (a.getAttribute('data-embed') === '1') {
          e.preventDefault();
          showEmbed(a.getAttribute('href'), a);
          return;
        }
        if (a.getAttribute('data-view') === 'dashboard') {
          e.preventDefault();
          showDashboard(a);
          return;
        }
        if (a.getAttribute('data-view') === 'fullstats') {
          e.preventDefault();
          showFullStats(a);
        }
      });
    });
    wireFullStats();
    async function load() {
      try {
        const r = await fetch('/stats/dashboard/data');
        if (!r.ok) throw new Error('HTTP ' + r.status);
        const j = await r.json();
        document.getElementById('err').textContent = '';
        const c = j.counters || {};
        document.getElementById('m-queries').textContent = c.total_queries != null ? c.total_queries : '—';
        document.getElementById('m-cache').textContent = c.total_cache_hits != null ? c.total_cache_hits : '—';
        const p = j.perf || {};
        document.getElementById('m-avg').textContent = p.avg_total_ms != null ? fmtMs(p.avg_total_ms) : '—';
        document.getElementById('m-up').textContent = p.outcome_upstream != null ? p.outcome_upstream : '—';
        const sum = j.summary || {};
        document.getElementById('m-cache-pct').textContent = sum.cache_hit_ratio != null && sum.cache_hit_ratio !== undefined ? fmtPctRatio(sum.cache_hit_ratio) : '—';
        document.getElementById('m-block-rate').textContent = sum.block_rate != null && sum.block_rate !== undefined ? fmtPctRatio(sum.block_rate) : '—';
        document.getElementById('m-blocks').textContent = c.total_blocks != null ? c.total_blocks : '—';
        document.getElementById('m-uptime').textContent = fmtUptimeSec(sum.uptime_seconds);
        const uh = sum.upstream || {};
        var upTxt = '—';
        if (uh.configured_rows > 0) {
          if (uh.checks_enabled) {
            upTxt = String(uh.healthy) + ' / ' + String(uh.configured_rows) + ' OK';
          } else {
            upTxt = String(uh.active || 0) + ' active';
          }
        } else {
          upTxt = String(uh.active || 0) + ' active';
        }
        document.getElementById('m-up-health').textContent = upTxt;
        document.getElementById('m-answered').textContent = c.total_queries_answered != null ? c.total_queries_answered : '—';
        document.getElementById('m-forwarded').textContent = c.total_queries_forwarded != null ? c.total_queries_forwarded : '—';
        document.getElementById('m-perf-total').textContent = p.total != null ? p.total : '—';
        document.getElementById('m-oc-local').textContent = p.outcome_local != null ? p.outcome_local : '—';
        document.getElementById('m-oc-cache').textContent = p.outcome_cache != null ? p.outcome_cache : '—';
        document.getElementById('m-oc-up').textContent = p.outcome_upstream != null ? p.outcome_upstream : '—';
        document.getElementById('m-oc-none').textContent = p.outcome_none != null ? p.outcome_none : '—';
        const build = j.build || {};
        document.getElementById('m-version').textContent = build.version ? esc(build.version) : '—';
        const fsRow = document.getElementById('fullstats-kpi-row');
        if (j.fullstats) {
          fsRow.style.display = '';
          const fs = j.fullstats;
          document.getElementById('m-fs-dom').textContent = fs.domains_total != null ? fs.domains_total : '—';
          document.getElementById('m-fs-req').textContent = fs.requesters_total != null ? fs.requesters_total : '—';
          document.getElementById('m-fs-dom-s').textContent = fs.domains_session != null ? fs.domains_session : '—';
          document.getElementById('m-fs-req-s').textContent = fs.requesters_session != null ? fs.requesters_session : '—';
        } else {
          fsRow.style.display = 'none';
        }
        const series = j.series || [];
        const labels = series.map(function(s) { return shortLabel(s.t); });
        const replies = series.map(function(s) { return s.replies || 0; });
        const avgs = series.map(function(s) { return s.avg_ms != null ? s.avg_ms : 0; });
        if (chartReplies && chartLatency) {
          chartReplies.data.labels = labels;
          chartReplies.data.datasets[0].data = replies;
          chartReplies.update('none');
          chartLatency.data.labels = labels;
          chartLatency.data.datasets[0].data = avgs;
          chartLatency.update('none');
        }
        const log = j.log || [];
        let h = '';
        for (let i = 0; i < log.length; i++) {
          const e = log[i];
          const oc = dotClass(e.outcome);
          const up = e.upstream ? ' · ' + esc(e.upstream) : '';
          h += '<div class="log-item"><span class="log-time">' + esc(relTime(e.at)) + '</span><div class="top"><span class="dot ' + oc + '"></span><div style="flex:1;min-width:0">';
          h += '<div class="log-query">' + esc(e.qtype) + ' <strong>' + esc(e.qname) + '</strong> · ' + fmtMs(e.duration_ms) + ' ms</div>';
          h += '<div class="log-meta">' + esc(e.outcome) + up + '</div>';
          h += '<div class="log-record">' + esc(e.record) + '</div>';
          h += '</div></div></div>';
        }
        if (!h) h = '<div class="log-item" style="color:var(--muted)">No resolutions yet — send DNS queries to this server.</div>';
        document.getElementById('log-root').innerHTML = h;
        const cw = document.getElementById('cluster-wrap');
        const cr = document.getElementById('cluster-root');
        if (j.cluster) {
          cw.style.display = 'block';
          const cl = j.cluster;
          if (!cl.enabled) {
            cr.innerHTML = '<span class="muted-link">Cluster is disabled.</span>';
          } else {
            let ch = '<div><strong>node_id</strong> ' + esc(cl.node_id) + ' · <strong>seq</strong> ' + esc(String(cl.local_seq)) + '</div>';
            ch += '<div style="margin-top:0.35rem"><strong>replica_only</strong> ' + esc(String(cl.replica_only)) + ' · <strong>cluster_admin</strong> ' + esc(String(cl.cluster_admin)) + '</div>';
            ch += '<div style="margin-top:0.35rem"><strong>dial</strong> ' + esc(cl.advertise_addr || cl.cluster_port_guess || '—') + '</div>';
            const peers = cl.peers || [];
            if (peers.length) {
              ch += '<table style="width:100%;margin-top:0.75rem;border-collapse:collapse;font-size:0.82rem"><thead><tr><th style="text-align:left;border-bottom:1px solid var(--border)">Peer</th><th style="text-align:left;border-bottom:1px solid var(--border)">Reachable</th><th style="text-align:left;border-bottom:1px solid var(--border)">Probe RTT ms</th><th style="text-align:left;border-bottom:1px solid var(--border)">Last error</th></tr></thead><tbody>';
              for (let i = 0; i < peers.length; i++) {
                const q = peers[i];
                ch += '<tr><td style="padding:0.35rem 0;border-bottom:1px solid var(--border)">' + esc(q.address) + '</td>';
                ch += '<td style="padding:0.35rem 0;border-bottom:1px solid var(--border)">' + esc(String(q.reachable)) + '</td>';
                ch += '<td style="padding:0.35rem 0;border-bottom:1px solid var(--border)">' + (q.last_probe_rtt_ms != null ? esc(String(q.last_probe_rtt_ms.toFixed(1))) : '—') + '</td>';
                ch += '<td style="padding:0.35rem 0;border-bottom:1px solid var(--border);word-break:break-all">' + esc(q.last_probe_error || q.last_outbound_error || '') + '</td></tr>';
              }
              ch += '</tbody></table>';
            } else {
              ch += '<div style="margin-top:0.5rem;color:var(--muted)">No peers configured.</div>';
            }
            cr.innerHTML = ch;
          }
        } else {
          cw.style.display = 'none';
        }
      } catch (e) {
        document.getElementById('err').textContent = 'Failed to load: ' + e.message;
      }
    }
    initCharts();
    showDashboard(null);
  </script>
</body>
</html>
`
