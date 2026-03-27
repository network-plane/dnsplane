// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"html/template"
	"net/http"

	"dnsplane/data"
)

type versionPageData struct {
	Version   string
	GoVersion string
	OS        string
	Arch      string
}

var versionPageTemplate = template.Must(template.New("versionPage").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dnsplane — Version</title>
  <style>
    :root {
      --bg: #0d1117;
      --surface: #161b22;
      --border: #30363d;
      --text: #e6edf3;
      --muted: #8b949e;
      --accent: #58a6ff;
    }
    * { box-sizing: border-box; }
    body {
      font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
      background: var(--bg);
      color: var(--text);
      margin: 0;
      padding: 1.5rem;
      line-height: 1.5;
      min-height: 100vh;
    }
    h1 { font-size: 1.35rem; font-weight: 600; margin: 0 0 0.75rem 0; }
    .muted { color: var(--muted); font-size: 0.9rem; margin: 0 0 1.25rem 0; }
    .muted a { color: var(--accent); text-decoration: none; }
    .muted a:hover { text-decoration: underline; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1rem 1.25rem;
      max-width: 32rem;
    }
    dl {
      margin: 0;
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.5rem 1.25rem;
      align-items: baseline;
      font-size: 0.95rem;
    }
    dt {
      color: var(--muted);
      font-weight: 600;
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    dd { margin: 0; font-variant-numeric: tabular-nums; word-break: break-word; }
  </style>
</head>
<body>
  <h1>Build</h1>
  <p class="muted"><a href="/version">JSON</a> — same fields as below</p>
  <div class="panel">
    <dl>
      <dt>Version</dt><dd>{{.Version}}</dd>
      <dt>Go</dt><dd>{{.GoVersion}}</dd>
      <dt>OS</dt><dd>{{.OS}}</dd>
      <dt>Arch</dt><dd>{{.Arch}}</dd>
    </dl>
  </div>
</body>
</html>
`))

// versionPageHandler serves human-readable build metadata for embedding in the dashboard.
func versionPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	b := BuildInfo()
	d := versionPageData{
		Version:   b["version"],
		GoVersion: b["go_version"],
		OS:        b["os"],
		Arch:      b["arch"],
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = versionPageTemplate.Execute(w, d)
}
