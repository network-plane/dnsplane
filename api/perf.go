// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"encoding/json"
	"html/template"
	"net/http"

	"dnsplane/data"
)

func perfStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rep := data.GetResolverPerfReport()
	writeJSON(w, http.StatusOK, rep)
}

func perfResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	data.ResetResolverPerf()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "Resolver perf counters reset"})
}

// buildPerfReportPayload returns the same JSON shape as GET /stats/perf (for WebSocket push).
func buildPerfReportPayload() map[string]any {
	rep := data.GetResolverPerfReport()
	b, err := json.Marshal(rep)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return m
}

var perfPageTemplate = template.Must(template.New("perf").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="dnsplane-api-token" content="">
  <title>dnsplane — Tuning</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js" crossorigin="anonymous"></script>
  <style>
    :root { --bg: #0f1419; --panel: #1a2332; --text: #e6edf3; --muted: #8b9cb3; --accent: #58a6ff; --border: #30363d; }
    body { font-family: ui-sans-serif, system-ui, sans-serif; background: var(--bg); color: var(--text); margin: 0; padding: 1.5rem; max-width: 56rem; margin-left: auto; margin-right: auto; }
    h1 { font-size: 1.25rem; margin-bottom: 0.5rem; }
    .muted { color: var(--muted); font-size: 0.875rem; margin-bottom: 1rem; }
    a { color: var(--accent); }
    .panel { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1rem; }
    table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
    th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); font-weight: 600; }
    .num { text-align: right; font-variant-numeric: tabular-nums; }
    button { background: var(--accent); color: #0f1419; border: 0; padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer; font-weight: 600; margin-right: 0.5rem; }
    button:hover { filter: brightness(1.1); }
    #err { color: #f85149; }
    .hist-sub { font-size: 0.8rem; color: var(--muted); font-weight: 600; margin: 0.75rem 0 0.35rem 0; }
    .chart-wrap { position: relative; height: 220px; margin-top: 0.35rem; }
    .perf-row-two {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 1rem;
      margin-bottom: 1rem;
      align-items: start;
    }
    .perf-row-two .panel { margin-bottom: 0; }
    @media (max-width: 42rem) {
      .perf-row-two { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <h1>Tuning (fast path)</h1>
  <p class="muted">Updates are pushed in real time over WebSocket. If WebSocket is unavailable or disconnects, this page falls back to HTTP refresh every 2s. · <a href="/stats/perf">JSON</a>
    · <button type="button" id="reset">Reset counters</button></p>
  <p id="err"></p>
  <div id="root"></div>
  <script>
    function esc(s) {
      if (s == null) return '';
      return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }
    function row(k, v) {
      return '<tr><th>' + esc(k) + '</th><td class="num">' + esc(v) + '</td></tr>';
    }
    var perfHistCharts = {};
    function perfChartCommon() {
      var grid = '#30363d';
      var tick = '#8b949e';
      return {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        scales: {
          x: {
            grid: { color: grid },
            ticks: { color: tick, maxRotation: 45, minRotation: 0, font: { size: 10 } }
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
    function perfDestroyHist(id) {
      if (perfHistCharts[id]) {
        perfHistCharts[id].destroy();
        delete perfHistCharts[id];
      }
    }
    function perfUpdateHist(canvasId, buckets, borderRgb, fillRgb) {
      var el = document.getElementById(canvasId);
      if (!el) return;
      var labels = [];
      var vals = [];
      (buckets || []).forEach(function(b) {
        labels.push(b.label);
        vals.push(Number(b.count));
      });
      perfHistCharts[canvasId] = new Chart(el, {
        type: 'bar',
        data: {
          labels: labels,
          datasets: [{
            label: 'Count',
            data: vals,
            backgroundColor: fillRgb,
            borderColor: borderRgb,
            borderWidth: 1
          }]
        },
        options: perfChartCommon()
      });
    }
    var perfPollTimer = null;
    var perfWebSocket = null;
    function stopPerfPoll() {
      if (perfPollTimer) {
        clearInterval(perfPollTimer);
        perfPollTimer = null;
      }
    }
    function applyPerfPayload(j) {
        document.getElementById('err').textContent = '';
        const a = j.a_resolve || {};
        let h = '<div class="perf-row-two">';
        h += '<div class="panel"><h2>Outcomes</h2><table>';
        h += row('Total resolves', a.total);
        h += row('Local (dnsrecords)', a.outcome_local);
        h += row('Cache (dnscache)', a.outcome_cache);
        h += row('Upstream', a.outcome_upstream);
        h += row('No answer', a.outcome_none);
        h += '</table></div>';
        h += '<div class="panel"><h2>Latency (all resolves)</h2><table>';
        h += row('Avg total (ms)', (a.avg_total_ms != null ? a.avg_total_ms.toFixed(3) : '—'));
        h += row('Avg prep (ms)', (a.avg_prep_ms != null ? a.avg_prep_ms.toFixed(3) : '—'));
        h += row('Max total (ms)', (a.max_total_ms != null ? a.max_total_ms.toFixed(3) : '—'));
        h += '</table></div>';
        h += '</div>';
        h += '<div class="panel"><h2>Cache-only path (dnscache hit)</h2><table>';
        h += row('Count', a.outcome_cache);
        h += row('Avg total (ms)', (a.avg_total_ms_cache_only != null ? a.avg_total_ms_cache_only.toFixed(3) : '—'));
        h += row('Max total (ms)', (a.max_total_ms_cache_only != null ? a.max_total_ms_cache_only.toFixed(3) : '—'));
        h += '</table><p class="muted" style="margin:0.5rem 0 0 0;font-size:0.8rem">If tail latency in dig is here, resolver/cache path. If flat here but dig spikes, compare Upstream count.</p>';
        h += '<p class="hist-sub">Total time (ms) — histogram</p><div class="chart-wrap"><canvas id="perf-hist-cache"></canvas></div></div>';
        if (a.outcome_upstream > 0) {
          h += '<div class="panel"><h2>Upstream path (any)</h2><table>';
          h += row('Count', a.outcome_upstream);
          h += row('Avg upstream wait (ms)', a.avg_upstream_wait_ms.toFixed(3));
          h += row('Avg slowest upstream (ms)', a.avg_max_upstream_ms.toFixed(3));
          h += row('Avg servers queried', a.avg_upstream_servers.toFixed(2));
          h += '</table><p class="hist-sub">Total time (ms) — histogram</p><div class="chart-wrap"><canvas id="perf-hist-upstream"></canvas></div></div>';
        }
        h += '<div class="panel"><h2>Histogram — all outcomes (mixed)</h2>';
        h += '<p class="hist-sub">Total time (ms) — histogram</p><div class="chart-wrap"><canvas id="perf-hist-total"></canvas></div></div>';
        const byQt = j.by_query_type || {};
        const qtKeys = Object.keys(byQt).sort();
        if (qtKeys.length > 0) {
          h += '<div class="panel"><h2>By QTYPE</h2><table><thead><tr><th>Type</th><th class="num">Total</th><th class="num">Avg ms</th><th class="num">Local</th><th class="num">Cache</th><th class="num">Upstream</th><th class="num">None</th></tr></thead><tbody>';
          qtKeys.forEach(function(k) {
            const q = byQt[k];
            h += '<tr><td>' + esc(k) + '</td><td class="num">' + q.total + '</td><td class="num">' +
              (q.avg_total_ms != null ? q.avg_total_ms.toFixed(3) : '—') + '</td><td class="num">' + q.outcome_local +
              '</td><td class="num">' + q.outcome_cache + '</td><td class="num">' + q.outcome_upstream +
              '</td><td class="num">' + q.outcome_none + '</td></tr>';
          });
          h += '</tbody></table></div>';
        }
        h += '<div class="panel muted"><p>' + esc(j.description || '') + '</p><ul>';
        (j.notes || []).forEach(function(n) { h += '<li>' + esc(n) + '</li>'; });
        h += '</ul><p>Since first A-resolve in this window: ' + esc(j.since_first_resolve || '—') + '</p></div>';
        perfDestroyHist('perf-hist-cache');
        perfDestroyHist('perf-hist-upstream');
        perfDestroyHist('perf-hist-total');
        document.getElementById('root').innerHTML = h;
        perfUpdateHist('perf-hist-cache', a.histogram_cache_only_ms, '#3fb950', 'rgba(63,185,80,0.45)');
        if (a.outcome_upstream > 0) {
          perfUpdateHist('perf-hist-upstream', a.histogram_upstream_ms, '#d29922', 'rgba(210,153,34,0.45)');
        }
        perfUpdateHist('perf-hist-total', a.histogram_total_ms, '#58a6ff', 'rgba(88,166,255,0.45)');
    }
    async function load() {
      try {
        const r = await fetch('/stats/perf');
        const j = await r.json();
        applyPerfPayload(j);
      } catch (e) {
        document.getElementById('err').textContent = String(e);
      }
    }
    function tryStartPerfWebSocket() {
      if (!window.WebSocket) {
        return;
      }
      if (perfWebSocket && (perfWebSocket.readyState === WebSocket.CONNECTING || perfWebSocket.readyState === WebSocket.OPEN)) {
        return;
      }
      var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      var u = new URL('/stats/dashboard/ws', location.href);
      u.protocol = proto;
      var meta = document.querySelector('meta[name="dnsplane-api-token"]');
      var tok = meta && meta.getAttribute('content');
      if (tok && String(tok).trim()) {
        u.searchParams.set('access_token', String(tok).trim());
      }
      try {
        perfWebSocket = new WebSocket(u.toString());
      } catch (e) {
        return;
      }
      perfWebSocket.onopen = function() {
        stopPerfPoll();
        try {
          perfWebSocket.send(JSON.stringify({ op: 'sub', stats: false, resolutions: false, perf: true }));
        } catch (e2) {}
      };
      perfWebSocket.onclose = function() {
        perfWebSocket = null;
        stopPerfPoll();
        load();
        perfPollTimer = setInterval(load, 2000);
      };
      perfWebSocket.onerror = function() {
        try { perfWebSocket.close(); } catch (e3) {}
      };
      perfWebSocket.onmessage = function(ev) {
        try {
          var msg = JSON.parse(ev.data);
          if (msg.v !== 1 || !msg.perf) {
            return;
          }
          applyPerfPayload(msg.perf);
        } catch (ex) {}
      };
    }
    document.getElementById('reset').onclick = async function() {
      try {
        const r = await fetch('/stats/perf/reset', { method: 'POST' });
        const j = await r.json();
        if (!r.ok) throw new Error(j.message || r.statusText);
        await load();
      } catch (e) {
        document.getElementById('err').textContent = String(e);
      }
    };
    load();
    perfPollTimer = setInterval(load, 2000);
    tryStartPerfWebSocket();
  </script>
</body>
</html>
`))

func perfPageHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStatsHTMLPage(w, r, data.StatsPerfPageHTMLEnabled()) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = perfPageTemplate.Execute(w, nil)
}
