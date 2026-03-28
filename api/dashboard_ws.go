// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dnsplane/data"

	"github.com/gorilla/websocket"
)

const (
	dashboardWSPushInterval  = 400 * time.Millisecond
	dashboardWSPingInterval  = 54 * time.Second
	dashboardWSReadWait      = 72 * time.Second
	dashboardWSMaxClients    = 32
	dashboardWSWriteDeadline = 10 * time.Second
)

var (
	dashboardWSUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 8192,
		CheckOrigin:     dashboardWSCheckOrigin,
	}
	dashboardWSClientCount int32
)

func dashboardWSCheckOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	host := r.Host
	if host == "" {
		return false
	}
	// Compare host part only (Origin is scheme://host[:port]).
	if strings.HasPrefix(origin, "http://") {
		rest := strings.TrimPrefix(origin, "http://")
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[:i]
		}
		return strings.EqualFold(rest, host)
	}
	if strings.HasPrefix(origin, "https://") {
		rest := strings.TrimPrefix(origin, "https://")
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[:i]
		}
		return strings.EqualFold(rest, host)
	}
	return false
}

// dashboardWSAuthorized checks API token for the WebSocket handshake.
// Browsers cannot set Authorization on WebSocket; allow access_token or token query param (use HTTPS in production).
func dashboardWSAuthorized(r *http.Request, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	if apiRequestAuthorized(r, want) {
		return true
	}
	q := r.URL.Query()
	for _, key := range []string{"access_token", "token"} {
		if got := strings.TrimSpace(q.Get(key)); got != "" && subtleStringEqual(got, want) {
			return true
		}
	}
	return false
}

// dashboardWebSocketHandler pushes dashboard + resolutions JSON when payloads change (bounded tick).
func dashboardWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !data.StatsDashboardHTMLEnabled() {
		http.NotFound(w, r)
		return
	}
	tok := strings.TrimSpace(data.GetInstance().GetResolverSettings().APIAuthToken)
	if tok != "" && !dashboardWSAuthorized(r, tok) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}` + "\n"))
		return
	}
	if atomic.LoadInt32(&dashboardWSClientCount) >= dashboardWSMaxClients {
		http.Error(w, "too many dashboard websocket clients", http.StatusServiceUnavailable)
		return
	}
	conn, err := dashboardWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	atomic.AddInt32(&dashboardWSClientCount, 1)
	defer atomic.AddInt32(&dashboardWSClientCount, -1)
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(dashboardWSReadWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(dashboardWSReadWait))
		return nil
	})
	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetWriteDeadline(time.Now().Add(dashboardWSWriteDeadline))
		return conn.WriteMessage(websocket.PongMessage, []byte(appData))
	})

	var subMu sync.Mutex
	subStats, subRes, subPerf := false, false, false

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m struct {
				Op          string `json:"op"`
				Stats       *bool  `json:"stats"`
				Resolutions *bool  `json:"resolutions"`
				Perf        *bool  `json:"perf"`
			}
			if json.Unmarshal(msg, &m) != nil || m.Op != "sub" {
				continue
			}
			subMu.Lock()
			if m.Stats != nil {
				subStats = *m.Stats
			}
			if m.Resolutions != nil {
				subRes = *m.Resolutions
			}
			if m.Perf != nil {
				subPerf = *m.Perf
			}
			subMu.Unlock()
		}
	}()

	ticker := time.NewTicker(dashboardWSPushInterval)
	pingTicker := time.NewTicker(dashboardWSPingInterval)
	defer ticker.Stop()
	defer pingTicker.Stop()

	var lastCombined []byte

	for {
		select {
		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(dashboardWSWriteDeadline))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ticker.C:
			subMu.Lock()
			wantStats := subStats
			wantRes := subRes
			wantPerf := subPerf
			subMu.Unlock()
			if !wantStats && !wantRes && !wantPerf {
				continue
			}

			env := map[string]any{"v": 1}
			if wantStats && data.StatsDashboardHTMLEnabled() {
				env["dashboard"] = buildDashboardPayload()
			}
			if wantRes && data.StatsDashboardHTMLEnabled() {
				env["resolutions"] = buildDashboardResolutionsPayload()
			}
			if wantPerf {
				env["perf"] = buildPerfReportPayload()
			}
			b, err := json.Marshal(env)
			if err != nil {
				continue
			}
			if bytes.Equal(b, lastCombined) {
				continue
			}
			lastCombined = append(lastCombined[:0], b...)

			_ = conn.SetWriteDeadline(time.Now().Add(dashboardWSWriteDeadline))
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		}
	}
}
