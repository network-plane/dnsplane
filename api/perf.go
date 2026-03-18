// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
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

var perfPageTemplate = template.Must(template.New("perf").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dnsplane — Resolver perf</title>
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
  </style>
</head>
<body>
  <h1>Resolver performance (fast path)</h1>
  <p class="muted">Auto-refresh every 2s · <a href="/stats/perf">JSON</a> · <a href="/stats/page">Stats</a>
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
    async function load() {
      try {
        const r = await fetch('/stats/perf');
        const j = await r.json();
        document.getElementById('err').textContent = '';
        const a = j.a_resolve || {};
        let h = '<div class="panel"><h2>Outcomes</h2><table>';
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
        if (a.outcome_upstream > 0) {
          h += '<div class="panel"><h2>Upstream path only</h2><table>';
          h += row('Avg upstream wait (ms)', a.avg_upstream_wait_ms.toFixed(3));
          h += row('Avg slowest upstream (ms)', a.avg_max_upstream_ms.toFixed(3));
          h += row('Avg servers queried', a.avg_upstream_servers.toFixed(2));
          h += '</table></div>';
        }
        h += '<div class="panel"><h2>Histogram — total time (ms)</h2><table><thead><tr><th>Bucket</th><th class="num">Count</th></tr></thead><tbody>';
        (a.histogram_total_ms || []).forEach(function(b) {
          h += '<tr><td>' + esc(b.label) + '</td><td class="num">' + b.count + '</td></tr>';
        });
        h += '</tbody></table></div>';
        h += '<div class="panel"><h2>Histogram — upstream-miss path only</h2><table><thead><tr><th>Bucket</th><th class="num">Count</th></tr></thead><tbody>';
        (a.histogram_upstream_ms || []).forEach(function(b) {
          h += '<tr><td>' + esc(b.label) + '</td><td class="num">' + b.count + '</td></tr>';
        });
        h += '</tbody></table></div>';
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
        document.getElementById('root').innerHTML = h;
      } catch (e) {
        document.getElementById('err').textContent = String(e);
      }
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
    setInterval(load, 2000);
  </script>
</body>
</html>
`))

func perfPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = perfPageTemplate.Execute(w, nil)
}
