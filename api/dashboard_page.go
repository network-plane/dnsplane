// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"net/http"

	"dnsplane/data"
)

func dashboardDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dnsData := data.GetInstance()
	stats := dnsData.GetStats()
	perf := data.GetResolverPerfReport()
	writeJSON(w, http.StatusOK, map[string]any{
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
		"series": data.GetDashboardSeries(),
		"log":    data.GetDashboardLogNewestFirst(200),
	})
}

func dashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}

// dashboardHTML is a self-contained dashboard (light theme, Chart.js from CDN).
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dnsplane — Dashboard</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js" crossorigin="anonymous"></script>
  <style>
    :root {
      --bg: #f4f6f9;
      --surface: #ffffff;
      --border: #e2e8f0;
      --text: #1e293b;
      --muted: #64748b;
      --accent: #16a34a;
      --accent-soft: #dcfce7;
      --sidebar-w: 220px;
      --shadow: 0 1px 3px rgba(15, 23, 42, 0.08);
      --radius: 12px;
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
      box-shadow: var(--shadow);
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
    nav a:hover { background: var(--bg); color: var(--text); }
    nav a.active {
      background: var(--accent-soft);
      color: var(--accent);
      font-weight: 600;
      border-right: 3px solid var(--accent);
    }
    main {
      flex: 1;
      padding: 1.5rem 1.75rem;
      overflow: auto;
    }
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
      box-shadow: var(--shadow);
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
      box-shadow: var(--shadow);
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
      box-shadow: var(--shadow);
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
    .dot.local { background: #16a34a; }
    .dot.cache { background: #2563eb; }
    .dot.upstream { background: #ea580c; }
    .dot.blocked { background: #dc2626; }
    .dot.none { background: #64748b; }
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
    .err { color: #dc2626; padding: 1rem; }
    .muted-link { color: var(--muted); font-size: 0.85rem; }
  </style>
</head>
<body>
  <div class="app">
    <aside>
      <div class="brand">dnsplane</div>
      <nav>
        <a class="active" href="/stats/dashboard">Dashboard</a>
        <a href="/stats/page">Stats</a>
        <a href="/stats/perf/page">Perf</a>
        <a href="/metrics">Metrics</a>
        <a href="/version">Version</a>
      </nav>
    </aside>
    <main>
      <h1>Dashboard</h1>
      <p class="muted-link" style="margin:-0.5rem 0 1rem 0">Live data · refreshes every 2s · <a href="/stats/dashboard/data">JSON</a></p>
      <div id="err" class="err"></div>
      <div class="metric-row">
        <div class="card"><h3>Total queries</h3><div class="value" id="m-queries">—</div><div class="sub">since start</div></div>
        <div class="card"><h3>Cache hits</h3><div class="value" id="m-cache">—</div><div class="sub">dnscache</div></div>
        <div class="card"><h3>Avg resolve (fast path)</h3><div class="value" id="m-avg">—</div><div class="sub">ms · A/AAAA perf</div></div>
        <div class="card"><h3>Upstream wins</h3><div class="value" id="m-up">—</div><div class="sub">outcome_upstream</div></div>
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
    let chartReplies, chartLatency;
    function initCharts() {
      const common = {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: { grid: { color: '#e2e8f0' }, ticks: { maxRotation: 45, font: { size: 10 } } },
          y: { beginAtZero: true, grid: { color: '#e2e8f0' } }
        },
        plugins: { legend: { display: false } }
      };
      chartReplies = new Chart(document.getElementById('chartReplies'), {
        type: 'line',
        data: { labels: [], datasets: [{ label: 'Replies', data: [], borderColor: '#16a34a', backgroundColor: 'rgba(22,163,74,0.1)', fill: true, tension: 0.25 }] },
        options: common
      });
      chartLatency = new Chart(document.getElementById('chartLatency'), {
        type: 'line',
        data: { labels: [], datasets: [{ label: 'Avg ms', data: [], borderColor: '#2563eb', backgroundColor: 'rgba(37,99,235,0.08)', fill: true, tension: 0.25 }] },
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
        if (!h) h = '<div class="log-item" style="color:#64748b">No resolutions yet — send DNS queries to this server.</div>';
        document.getElementById('log-root').innerHTML = h;
      } catch (e) {
        document.getElementById('err').textContent = 'Failed to load: ' + e.message;
      }
    }
    initCharts();
    load();
    setInterval(load, 2000);
  </script>
</body>
</html>
`
