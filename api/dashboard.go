package api

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"time"

	"dnsplane/data"
	"dnsplane/fullstats"
)

const statsPageLimit = 10

// dashboardData is the struct passed to the stats page template.
type dashboardData struct {
	TotalQueries          int
	TotalCacheHits        int
	TotalBlocks           int
	TotalQueriesForwarded int
	TotalQueriesAnswered  int
	ServerStartTime       string
	Uptime                string

	RecordsCount int
	ServersCount int
	CacheCount   int
	AdblockCount int

	Ready     bool
	APIUp     bool
	DNSUp     bool
	TUIClient struct {
		Connected bool
		Addr      string
		Since     string
	}
	Listeners struct {
		DNSPort          string
		APIPort          string
		APIEnabled       bool
		ClientSocketPath string
		ClientTCPAddress string
	}

	FullStatsEnabled bool
	RequestersCount  int
	DomainsCount     int
	StatsLimit       int   // limit used for top requesters/domains (e.g. 10 or 100)
	TopRequesters    []struct {
		IP        string
		Total     uint64
		FirstSeen string
	}
	TopDomains []struct {
		Key                 string
		Count               uint64
		FirstSeen, LastSeen string
	}
}

var statsPageTemplate = template.Must(template.New("stats").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dnsplane Stats</title>
  <style>
    :root {
      --bg: #0d1117;
      --bg-panel: #161b22;
      --bg-hover: #21262d;
      --border: #30363d;
      --text: #e6edf3;
      --text-muted: #8b949e;
      --accent: #58a6ff;
      --success: #3fb950;
      --warning: #d29922;
      --danger: #f85149;
    }
    * { box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif;
      background: var(--bg);
      color: var(--text);
      margin: 0;
      padding: 1.5rem;
      line-height: 1.5;
      min-height: 100vh;
    }
    h1 {
      font-size: 1.5rem;
      font-weight: 600;
      margin: 0 0 1.5rem 0;
      color: var(--text);
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 1rem;
    }
    .panel {
      background: var(--bg-panel);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1rem 1.25rem;
      overflow: hidden;
    }
    .panel h2 {
      font-size: 0.875rem;
      font-weight: 600;
      color: var(--text-muted);
      text-transform: uppercase;
      letter-spacing: 0.03em;
      margin: 0 0 0.75rem 0;
      padding-bottom: 0.5rem;
      border-bottom: 1px solid var(--border);
    }
    .panel ul { margin: 0; padding: 0; list-style: none; }
    .panel li {
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      padding: 0.35rem 0;
      border-bottom: 1px solid var(--border);
    }
    .panel li:last-child { border-bottom: none; }
    .panel .key { color: var(--text-muted); }
    .panel .val { font-variant-numeric: tabular-nums; color: var(--text); }
    .panel.wide { grid-column: 1 / -1; }
    .status-dot {
      display: inline-block;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      margin-right: 0.5rem;
    }
    .status-dot.ok { background: var(--success); }
    .status-dot.fail { background: var(--danger); }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.875rem;
    }
    th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid var(--border); }
    th { color: var(--text-muted); font-weight: 600; }
    tr:last-child td { border-bottom: none; }
    tr:hover td { background: var(--bg-hover); }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .muted { color: var(--text-muted); font-size: 0.875rem; margin-top: 1rem; }
  </style>
</head>
<body>
  <h1>dnsplane Stats</h1>
  <div class="grid">
    <div class="panel">
      <h2>Resolver</h2>
      <ul>
        <li><span class="key">Queries</span><span class="val">{{.TotalQueries}}</span></li>
        <li><span class="key">Cache hits</span><span class="val">{{.TotalCacheHits}}</span></li>
        <li><span class="key">Blocks</span><span class="val">{{.TotalBlocks}}</span></li>
        <li><span class="key">Forwarded</span><span class="val">{{.TotalQueriesForwarded}}</span></li>
        <li><span class="key">Answered</span><span class="val">{{.TotalQueriesAnswered}}</span></li>
        <li><span class="key">Uptime</span><span class="val">{{.Uptime}}</span></li>
        <li><span class="key">Started</span><span class="val">{{.ServerStartTime}}</span></li>
      </ul>
    </div>
    <div class="panel">
      <h2>Data</h2>
      <ul>
        <li><span class="key">Records</span><span class="val">{{.RecordsCount}}</span></li>
        <li><span class="key">Upstream servers</span><span class="val">{{.ServersCount}}</span></li>
        <li><span class="key">Cache entries</span><span class="val">{{.CacheCount}}</span></li>
        <li><span class="key">Adblock domains</span><span class="val">{{.AdblockCount}}</span></li>
      </ul>
    </div>
    <div class="panel">
      <h2>Status</h2>
      <ul>
        <li><span class="key"><span class="status-dot {{if .Ready}}ok{{else}}fail{{end}}"></span>Ready</span><span class="val">{{if .Ready}}Yes{{else}}No{{end}}</span></li>
        <li><span class="key"><span class="status-dot {{if .APIUp}}ok{{else}}fail{{end}}"></span>API</span><span class="val">{{if .APIUp}}Up{{else}}Down{{end}}</span></li>
        <li><span class="key"><span class="status-dot {{if .DNSUp}}ok{{else}}fail{{end}}"></span>DNS</span><span class="val">{{if .DNSUp}}Up{{else}}Down{{end}}</span></li>
        <li><span class="key">TUI client</span><span class="val">{{if .TUIClient.Connected}}{{.TUIClient.Addr}} (since {{.TUIClient.Since}}){{else}}—{{end}}</span></li>
        <li><span class="key">DNS port</span><span class="val">{{.Listeners.DNSPort}}</span></li>
        <li><span class="key">API port</span><span class="val">{{.Listeners.APIPort}}</span></li>
        <li><span class="key">API enabled</span><span class="val">{{if .Listeners.APIEnabled}}Yes{{else}}No{{end}}</span></li>
        <li><span class="key">Client socket</span><span class="val">{{.Listeners.ClientSocketPath}}</span></li>
        <li><span class="key">Client TCP</span><span class="val">{{.Listeners.ClientTCPAddress}}</span></li>
      </ul>
    </div>
    {{if .FullStatsEnabled}}
    <div class="panel wide">
      <h2>Full stats</h2>
      <ul>
        <li><span class="key">Requesters</span><span class="val">{{.RequestersCount}}</span></li>
        <li><span class="key">Domain:type entries</span><span class="val">{{.DomainsCount}}</span></li>
      </ul>
      <p class="muted">
        {{if le .StatsLimit 10}}<a href="/stats/page?full=100">Show all</a> (up to 100){{else}}<a href="/stats/page">Show top 10</a>{{end}}
      </p>
      {{if .TopRequesters}}
      <p class="muted">Top {{len .TopRequesters}} requesters by total requests</p>
      <table>
        <thead><tr><th>IP</th><th>Total</th><th>First seen</th></tr></thead>
        <tbody>
          {{range .TopRequesters}}<tr><td>{{.IP}}</td><td>{{.Total}}</td><td>{{.FirstSeen}}</td></tr>{{end}}
        </tbody>
      </table>
      {{end}}
      {{if .TopDomains}}
      <p class="muted">Top {{len .TopDomains}} domains (name:type) by request count</p>
      <table>
        <thead><tr><th>Domain (name:type)</th><th>Count</th><th>First seen</th><th>Last seen</th></tr></thead>
        <tbody>
          {{range .TopDomains}}<tr><td>{{.Key}}</td><td>{{.Count}}</td><td>{{.FirstSeen}}</td><td>{{.LastSeen}}</td></tr>{{end}}
        </tbody>
      </table>
      {{end}}
    </div>
    {{end}}
  </div>
  <p class="muted">Read-only dashboard · JSON: <a href="/stats">/stats</a> · Prometheus: <a href="/metrics">/metrics</a></p>
</body>
</html>
`))

// statsPageHandler serves a dark-themed read-only stats dashboard with optional full_stats.
func statsPageHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	stats := dnsData.GetStats()

	uptime := "—"
	if !stats.ServerStartTime.IsZero() {
		d := time.Since(stats.ServerStartTime)
		uptime = roundDuration(d)
	}

	blockList := dnsData.GetBlockList()
	adblockCount := 0
	if blockList != nil {
		adblockCount = blockList.Count()
	}

	apiServerMu.Lock()
	state := apiState
	tracker := apiFullStatsTracker
	apiServerMu.Unlock()

	apiUp := state != nil && state.APIRunning()
	dnsUp := state != nil && state.ServerStatus()
	ready := apiUp && dnsUp

	tuiAddr, tuiSince := "", time.Time{}
	if state != nil {
		tuiAddr, tuiSince = state.GetTUIClientInfo()
	}

	listeners := struct {
		DNSPort          string
		APIPort          string
		APIEnabled       bool
		ClientSocketPath string
		ClientTCPAddress string
	}{}
	if state != nil {
		ls := state.ListenerSnapshot()
		listeners.DNSPort = ls.DNSPort
		listeners.APIPort = ls.APIPort
		listeners.APIEnabled = ls.APIEnabled
		listeners.ClientSocketPath = ls.ClientSocketPath
		listeners.ClientTCPAddress = ls.ClientTCPAddress
	}

	data := dashboardData{
		TotalQueries:          stats.TotalQueries,
		TotalCacheHits:        stats.TotalCacheHits,
		TotalBlocks:           stats.TotalBlocks,
		TotalQueriesForwarded: stats.TotalQueriesForwarded,
		TotalQueriesAnswered:  stats.TotalQueriesAnswered,
		ServerStartTime:       stats.ServerStartTime.Format(time.RFC3339),
		Uptime:                uptime,
		RecordsCount:          len(dnsData.GetRecords()),
		ServersCount:          len(dnsData.GetServers()),
		CacheCount:            len(dnsData.GetCacheRecords()),
		AdblockCount:          adblockCount,
		Ready:                 ready,
		APIUp:                 apiUp,
		DNSUp:                 dnsUp,
		Listeners:             listeners,
	}
	data.TUIClient.Connected = tuiAddr != ""
	data.TUIClient.Addr = tuiAddr
	if !tuiSince.IsZero() {
		data.TUIClient.Since = tuiSince.Format(time.RFC3339)
	}

	limit := statsPageLimit
	if n := r.URL.Query().Get("full"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	if tracker != nil {
		data.FullStatsEnabled = true
		data.StatsLimit = limit
		reqs, _ := tracker.GetAllRequesters()
		doms, _ := tracker.GetAllRequests()
		if reqs != nil {
			data.RequestersCount = len(reqs)
			data.TopRequesters = topRequestersForDashboard(reqs, limit)
		}
		if doms != nil {
			data.DomainsCount = len(doms)
			data.TopDomains = topDomainsForDashboard(doms, limit)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := statsPageTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

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
	Key                 string
	Count               uint64
	FirstSeen, LastSeen string
} {
	type row struct {
		key         string
		count       uint64
		first, last time.Time
	}
	var rows []row
	for key, st := range all {
		rows = append(rows, row{key: key, count: st.Count, first: st.FirstSeen, last: st.LastSeen})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].count > rows[j].count })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]struct {
		Key                 string
		Count               uint64
		FirstSeen, LastSeen string
	}, len(rows))
	for i, r := range rows {
		out[i].Key = r.key
		out[i].Count = r.count
		out[i].FirstSeen = r.first.Format(time.RFC3339)
		out[i].LastSeen = r.last.Format(time.RFC3339)
	}
	return out
}

func roundDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	if d < 24*time.Hour {
		return d.Round(time.Hour).String()
	}
	days := int(d / (24 * time.Hour))
	rem := d % (24 * time.Hour)
	if rem == 0 {
		return strconv.Itoa(days) + "d"
	}
	return strconv.Itoa(days) + "d " + rem.Round(time.Hour).String()
}
