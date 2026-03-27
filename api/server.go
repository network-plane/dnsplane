// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
	"dnsplane/dnsservers"
	"dnsplane/fullstats"
	"dnsplane/ratelimit"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	apiServerMu         sync.Mutex
	apiServer           *http.Server
	apiState            *daemon.State
	apiFullStatsTracker *fullstats.Tracker
)

// appVersion is injected from main via SetAppVersion.
var appVersion string

// SetAppVersion registers the application release string for /ready, /version, and /stats JSON.
func SetAppVersion(v string) {
	appVersion = v
}

// BuildInfo returns version and runtime metadata for JSON APIs.
func BuildInfo() map[string]string {
	return map[string]string{
		"version":    appVersion,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
}

// SetFullStatsTracker sets the full statistics tracker for /stats and /metrics (optional).
func SetFullStatsTracker(t *fullstats.Tracker) {
	apiServerMu.Lock()
	defer apiServerMu.Unlock()
	apiFullStatsTracker = t
}

// fullStatsCounts holds requesters and domains counts for a scope (session or total).
type fullStatsCounts struct {
	RequestersCount int `json:"requesters_count"`
	DomainsCount    int `json:"domains_count"`
}

// getFullStatsCounts returns total (DB) and session (since process start) full_stats counts.
// Caller must not hold apiServerMu.
func getFullStatsCounts() (total, session fullStatsCounts, ok bool) {
	apiServerMu.Lock()
	tracker := apiFullStatsTracker
	apiServerMu.Unlock()
	if tracker == nil {
		return fullStatsCounts{}, fullStatsCounts{}, false
	}
	reqsTotal, _ := tracker.GetAllRequesters()
	domsTotal, _ := tracker.GetAllRequests()
	reqsSession, _ := tracker.GetSessionRequesters()
	domsSession, _ := tracker.GetSessionRequests()
	if reqsTotal != nil {
		total.RequestersCount = len(reqsTotal)
	}
	if domsTotal != nil {
		total.DomainsCount = len(domsTotal)
	}
	if reqsSession != nil {
		session.RequestersCount = len(reqsSession)
	}
	if domsSession != nil {
		session.DomainsCount = len(domsSession)
	}
	return total, session, true
}

// RouteRegistrar registers HTTP routes on the supplied Chi router.
type RouteRegistrar func(chi.Router)

// Start runs the REST API in the background. registrar nil uses RegisterDNSRoutes; opts nil uses zero defaults;
// logger nil skips structured API file logging.
func Start(state *daemon.State, port string, opts *ListenOptions, registrar RouteRegistrar, logger *slog.Logger) {
	if state == nil {
		logAPIWarn(logger, "missing daemon state; cannot start API")
		return
	}
	trimmed := strings.TrimSpace(port)
	if trimmed == "" {
		logAPIWarn(logger, "invalid port; refusing to start")
		return
	}
	if state.APIRunning() {
		if logger != nil {
			logger.Info("API server already running; skipping start")
		}
		return
	}
	if registrar == nil {
		registrar = RegisterDNSRoutes
	}
	if opts == nil {
		opts = &ListenOptions{}
	}

	apiServerMu.Lock()
	apiState = state
	apiServerMu.Unlock()
	state.SetAPIRunning(true)
	bindIP := strings.TrimSpace(opts.BindIP)
	addr := ":" + trimmed
	if bindIP != "" {
		addr = net.JoinHostPort(bindIP, trimmed)
	}
	if logger != nil {
		logger.Info("API server starting", "addr", addr, "tls", opts.TLSCertFile != "" && opts.TLSKeyFile != "")
	}
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	var lim *ratelimit.PerIP
	if opts.RateLimitRPS > 0 {
		burst := opts.RateLimitBurst
		if burst <= 0 {
			burst = 20
		}
		lim = ratelimit.NewPerIP(opts.RateLimitRPS, burst)
	}
	if lim != nil {
		router.Use(rateLimitMiddleware(lim))
	}
	router.Use(apiAuthMiddleware())
	registrar(router)

	srv := &http.Server{Addr: addr, Handler: router}
	apiServerMu.Lock()
	apiServer = srv
	apiServerMu.Unlock()
	go func() {
		defer state.SetAPIRunning(false)
		var err error
		if strings.TrimSpace(opts.TLSCertFile) != "" && strings.TrimSpace(opts.TLSKeyFile) != "" {
			err = srv.ListenAndServeTLS(opts.TLSCertFile, opts.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logAPIError(logger, "API server stopped with error", "error", err)
		}
	}()
}

// Stop shuts down the API server and updates state. No-op if not running.
func Stop(state *daemon.State) {
	apiServerMu.Lock()
	srv := apiServer
	apiServer = nil
	apiState = nil
	apiServerMu.Unlock()
	if srv == nil {
		return
	}
	_ = srv.Shutdown(context.Background())
	if state != nil {
		state.SetAPIRunning(false)
	}
}

func logAPIWarn(logger *slog.Logger, msg string, keyValues ...any) {
	if logger != nil {
		logger.Warn(msg, keyValues...)
	}
}

func logAPIError(logger *slog.Logger, msg string, keyValues ...any) {
	if logger != nil {
		logger.Error(msg, keyValues...)
	}
}

// RegisterDNSRoutes wires up the default DNS-related REST handlers.
func RegisterDNSRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/health", healthHandler)
	router.Get("/ready", readyHandler)
	router.Get("/version", versionHandler)
	router.Get("/version/page", versionPageHandler)
	router.Get("/dns/records", listRecordsHandler)
	router.Post("/dns/records", addRecordHandler)
	router.Put("/dns/records", updateRecordHandler)
	router.Delete("/dns/records", deleteRecordHandler)
	router.Get("/dns/servers", listServersHandler)
	router.Get("/dns/upstreams/health", upstreamHealthHandler)
	router.Post("/dns/servers", addServerHandler)
	router.Put("/dns/servers/{address}", updateServerHandler)
	router.Delete("/dns/servers/{address}", deleteServerHandler)
	router.Get("/adblock/domains", listAdblockDomainsHandler)
	router.Get("/adblock/sources", listAdblockSourcesHandler)
	router.Post("/adblock/domains", addAdblockDomainsHandler)
	router.Delete("/adblock/domains", deleteAdblockDomainsHandler)
	router.Post("/adblock/clear", clearAdblockHandler)
	router.Get("/cache", getCacheHandler)
	router.Post("/cache/clear", clearCacheHandler)
	router.Delete("/cache", clearCacheHandler)
	router.Get("/stats", statsHandler)
	router.Get("/metrics", metricsHandler)
	router.Get("/stats/dashboard", dashboardPageHandler)
	router.Get("/stats/dashboard/icon", dashboardIconSVGHandler)
	router.Get("/stats/dashboard/data", dashboardDataHandler)
	router.Get("/stats/dashboard/resolutions", dashboardResolutionsHandler)
	router.Post("/stats/dashboard/resolutions/purge", dashboardResolutionsPurgeHandler)
	router.Get("/stats/dashboard/fullstats/data", fullstatsBrowseHandler)
	router.Get("/stats/perf", perfStatsHandler)
	router.Post("/stats/perf/reset", perfResetHandler)
	router.Get("/stats/perf/page", perfPageHandler)
}

// healthHandler returns 200 when the API is up. No dependency on DNS listener.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// readyHandler returns JSON with API, DNS, TUI client, and listener status. 200 when DNS is up, 503 otherwise.
func readyHandler(w http.ResponseWriter, r *http.Request) {
	apiServerMu.Lock()
	state := apiState
	apiServerMu.Unlock()

	apiUp := state != nil && state.APIRunning()
	dnsUp := state != nil && state.ServerStatus()
	ready := apiUp && dnsUp

	tuiAddr, tuiSince := "", time.Time{}
	if state != nil {
		tuiAddr, tuiSince = state.GetTUIClientInfo()
	}
	tuiConnected := tuiAddr != ""

	listeners := map[string]any{}
	if state != nil {
		ls := state.ListenerSnapshot()
		listeners = map[string]any{
			"dns_port":           ls.DNSPort,
			"api_port":           ls.APIPort,
			"api_enabled":        ls.APIEnabled,
			"client_socket_path": ls.ClientSocketPath,
			"client_tcp_address": ls.ClientTCPAddress,
		}
	}

	resp := map[string]any{
		"ready":     ready,
		"api":       apiUp,
		"dns":       dnsUp,
		"listeners": listeners,
		"build":     BuildInfo(),
	}
	tuiObj := map[string]any{"connected": tuiConnected}
	if tuiConnected {
		tuiObj["addr"] = tuiAddr
		tuiObj["since"] = tuiSince.Format(time.RFC3339)
	}
	resp["tui_client"] = tuiObj

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

// versionHandler returns build metadata (version, Go, OS, arch) as JSON.
func versionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, BuildInfo())
}

// listServersHandler returns configured upstreams plus optional health when checks are enabled.
func listServersHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	servers := dnsData.GetServers()
	health, enabled := dnsData.UpstreamHealthStatuses()
	cfg := dnsData.GetResolverSettings()
	interval := cfg.UpstreamHealthCheckIntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	failures := cfg.UpstreamHealthCheckFailures
	if failures <= 0 {
		failures = 3
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"servers":                                servers,
		"upstream_health_check_enabled":          enabled,
		"upstream_health_check_interval_seconds": interval,
		"upstream_health_check_failures":         failures,
		"upstream_health":                        health,
	})
}

// upstreamHealthHandler returns only upstream health probe state (read-only).
func upstreamHealthHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	health, enabled := dnsData.UpstreamHealthStatuses()
	cfg := dnsData.GetResolverSettings()
	interval := cfg.UpstreamHealthCheckIntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	failures := cfg.UpstreamHealthCheckFailures
	if failures <= 0 {
		failures = 3
	}
	q := strings.TrimSpace(cfg.UpstreamHealthCheckQueryName)
	if q == "" {
		q = "google.com."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"upstream_health_check_enabled":          enabled,
		"upstream_health_check_interval_seconds": interval,
		"upstream_health_check_failures":         failures,
		"upstream_health_check_query_name":       q,
		"upstreams":                              health,
	})
}

// resolverStatsMap builds resolver counters for /stats JSON (same map used for session and total scopes).
func resolverStatsMap(stats data.DNSStats) map[string]any {
	return map[string]any{
		"total_queries":           stats.TotalQueries,
		"total_cache_hits":        stats.TotalCacheHits,
		"total_blocks":            stats.TotalBlocks,
		"total_queries_forwarded": stats.TotalQueriesForwarded,
		"total_queries_answered":  stats.TotalQueriesAnswered,
		"server_start_time":       stats.ServerStartTime.Format(time.RFC3339),
	}
}

// statsHandler returns session/total scopes and build info. Resolver counters are in-memory only, so both scopes match.
func statsHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	stats := dnsData.GetStats()

	resolver := resolverStatsMap(stats)
	sessionMap := map[string]any{"resolver": resolver}
	totalMap := map[string]any{"resolver": resolver}

	if total, session, ok := getFullStatsCounts(); ok {
		sessionMap["full_stats"] = map[string]any{
			"enabled":          true,
			"requesters_count": session.RequestersCount,
			"domains_count":    session.DomainsCount,
		}
		totalMap["full_stats"] = map[string]any{
			"enabled":          true,
			"requesters_count": total.RequestersCount,
			"domains_count":    total.DomainsCount,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session": sessionMap,
		"total":   totalMap,
		"build":   BuildInfo(),
	})
}

// prometheusMetric describes a single Prometheus metric for loop-based output.
type prometheusMetric struct {
	help, typeName, name string
	value                any
}

// metricsHandler returns Prometheus text format (counters/gauges for resolver and optional full_stats).
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	stats := dnsData.GetStats()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	metrics := []prometheusMetric{
		{"Total DNS queries received", "counter", "dnsplane_queries_total", stats.TotalQueries},
		{"Total cache hits", "counter", "dnsplane_cache_hits_total", stats.TotalCacheHits},
		{"Total adblock blocks", "counter", "dnsplane_blocks_total", stats.TotalBlocks},
		{"Total queries forwarded to upstreams", "counter", "dnsplane_queries_forwarded_total", stats.TotalQueriesForwarded},
		{"Total queries answered", "counter", "dnsplane_queries_answered_total", stats.TotalQueriesAnswered},
		{"Server start time (Unix)", "gauge", "dnsplane_server_start_time_seconds", float64(stats.ServerStartTime.Unix())},
	}
	for _, m := range metrics {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
		_, _ = fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typeName)
		switch v := m.value.(type) {
		case int:
			_, _ = fmt.Fprintf(w, "%s %d\n", m.name, v)
		case float64:
			_, _ = fmt.Fprintf(w, "%s %.0f\n", m.name, v)
		default:
			_, _ = fmt.Fprintf(w, "%s %v\n", m.name, v)
		}
	}

	if total, session, ok := getFullStatsCounts(); ok {
		fullStatsMetrics := []prometheusMetric{
			{"Number of distinct requesters in full_stats (total)", "gauge", "dnsplane_fullstats_requesters_count_total", total.RequestersCount},
			{"Number of distinct domain:type entries in full_stats (total)", "gauge", "dnsplane_fullstats_domains_count_total", total.DomainsCount},
			{"Number of distinct requesters in full_stats (session)", "gauge", "dnsplane_fullstats_requesters_count_session", session.RequestersCount},
			{"Number of distinct domain:type entries in full_stats (session)", "gauge", "dnsplane_fullstats_domains_count_session", session.DomainsCount},
		}
		for _, m := range fullStatsMetrics {
			_, _ = fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
			_, _ = fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typeName)
			_, _ = fmt.Fprintf(w, "%s %d\n", m.name, m.value.(int))
		}
	}

	_, _ = fmt.Fprintln(w)
	data.WriteResolverPerfPrometheus(w)
	_, _ = fmt.Fprintln(w)
	data.WriteDNSAbusePrometheus(w)
}

type AddRecordRequest struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Value string  `json:"value"`
	TTL   *uint32 `json:"ttl,omitempty"`
}

func (r AddRecordRequest) toDNSRecord() dnsrecords.DNSRecord {
	ttl := uint32(3600)
	if r.TTL != nil && *r.TTL > 0 {
		ttl = *r.TTL
	}
	return dnsrecords.DNSRecord{
		Name:  strings.TrimSpace(r.Name),
		Type:  strings.TrimSpace(r.Type),
		Value: strings.TrimSpace(r.Value),
		TTL:   ttl,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func addRecordHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	var request AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Type) == "" || strings.TrimSpace(request.Value) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}

	record := request.toDNSRecord()
	records := dnsData.GetRecords()
	updated, messages, err := dnsrecords.AddRecord(record, records, false)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsrecords.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractRecordMessages(messages)})
		return
	}
	if err := dnsData.UpdateRecords(updated); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "record added", "messages": extractRecordMessages(messages)})
}

func listRecordsHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	records := dnsData.GetRecords()
	result, err := dnsrecords.List(records, []string{})
	if errors.Is(err, dnsrecords.ErrHelpRequested) {
		writeJSON(w, http.StatusOK, map[string]any{"messages": extractRecordMessages(result.Messages)})
		return
	}
	resp := map[string]any{
		"records":  result.Records,
		"detailed": result.Detailed,
	}
	if result.Filter != "" {
		resp["filter"] = result.Filter
	}
	if len(result.Messages) > 0 {
		resp["messages"] = extractRecordMessages(result.Messages)
	}
	writeJSON(w, http.StatusOK, resp)
}

func extractRecordMessages(msgs []dnsrecords.Message) []string {
	if len(msgs) == 0 {
		return nil
	}
	res := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		res = append(res, msg.Text)
	}
	return res
}

func updateRecordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	dnsData := data.GetInstance()
	var request AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Type) == "" || strings.TrimSpace(request.Value) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, type, and value are required"})
		return
	}
	record := request.toDNSRecord()
	records := dnsData.GetRecords()
	updated, messages, err := dnsrecords.AddRecord(record, records, true)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsrecords.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractRecordMessages(messages)})
		return
	}
	if err := dnsData.UpdateRecords(updated); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "record updated", "messages": extractRecordMessages(messages)})
}

func deleteRecordHandler(w http.ResponseWriter, r *http.Request) {
	var name, recordType, value string
	if r.Method == http.MethodDelete && r.URL.RawQuery != "" {
		q := r.URL.Query()
		name = q.Get("name")
		recordType = q.Get("type")
		value = q.Get("value")
	}
	if (name == "" && recordType == "" && value == "") && r.Body != nil {
		var body struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if name == "" {
				name = body.Name
			}
			if recordType == "" {
				recordType = body.Type
			}
			if value == "" {
				value = body.Value
			}
		}
	}
	name = strings.TrimSpace(name)
	recordType = strings.TrimSpace(recordType)
	value = strings.TrimSpace(value)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	var args []string
	if recordType != "" && value != "" {
		args = []string{name, recordType, value}
	} else if value != "" {
		args = []string{name, value}
	} else {
		args = []string{name}
	}
	dnsData := data.GetInstance()
	records := dnsData.GetRecords()
	updated, messages, err := dnsrecords.Remove(args, records)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsrecords.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractRecordMessages(messages)})
		return
	}
	if err := dnsData.UpdateRecords(updated); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "record deleted", "messages": extractRecordMessages(messages)})
}

type addServerRequest struct {
	Address         string   `json:"address"`
	Port            string   `json:"port"`
	Active          *bool    `json:"active,omitempty"`
	LocalResolver   *bool    `json:"local_resolver,omitempty"`
	AdBlocker       *bool    `json:"adblocker,omitempty"`
	DomainWhitelist []string `json:"domain_whitelist,omitempty"`
}

func serverRequestToArgs(req addServerRequest) []string {
	if req.Port == "" {
		req.Port = "53"
	}
	args := []string{strings.TrimSpace(req.Address), strings.TrimSpace(req.Port)}
	if req.Active != nil {
		args = append(args, fmt.Sprintf("active:%v", *req.Active))
	}
	if req.LocalResolver != nil {
		args = append(args, fmt.Sprintf("localresolver:%v", *req.LocalResolver))
	}
	if req.AdBlocker != nil {
		args = append(args, fmt.Sprintf("adblocker:%v", *req.AdBlocker))
	}
	if len(req.DomainWhitelist) > 0 {
		args = append(args, "whitelist:"+strings.Join(req.DomainWhitelist, ","))
	}
	return args
}

func addServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	var req addServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	req.Address = strings.TrimSpace(req.Address)
	if req.Address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address is required"})
		return
	}
	dnsData := data.GetInstance()
	servers := dnsData.GetServers()
	args := serverRequestToArgs(req)
	updated, messages, err := dnsservers.Add(args, servers)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsservers.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractServerMessages(messages)})
		return
	}
	dnsData.UpdateServers(updated)
	writeJSON(w, http.StatusCreated, map[string]any{"status": "server added", "messages": extractServerMessages(messages)})
}

func updateServerHandler(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if u, err := url.PathUnescape(address); err == nil {
		address = u
	}
	address = strings.TrimSpace(address)
	if address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address is required"})
		return
	}
	var req addServerRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	req.Address = address
	if req.Port == "" {
		req.Port = "53"
	}
	dnsData := data.GetInstance()
	servers := dnsData.GetServers()
	args := serverRequestToArgs(req)
	updated, messages, err := dnsservers.Update(args, servers)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsservers.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractServerMessages(messages)})
		return
	}
	dnsData.UpdateServers(updated)
	writeJSON(w, http.StatusOK, map[string]any{"status": "server updated", "messages": extractServerMessages(messages)})
}

func deleteServerHandler(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if u, err := url.PathUnescape(address); err == nil {
		address = u
	}
	address = strings.TrimSpace(address)
	if address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address is required"})
		return
	}
	dnsData := data.GetInstance()
	servers := dnsData.GetServers()
	updated, messages, err := dnsservers.Remove([]string{address}, servers)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, dnsservers.ErrInvalidArgs) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "messages": extractServerMessages(messages)})
		return
	}
	dnsData.UpdateServers(updated)
	writeJSON(w, http.StatusOK, map[string]any{"status": "server removed", "messages": extractServerMessages(messages)})
}

func extractServerMessages(msgs []dnsservers.Message) []string {
	if len(msgs) == 0 {
		return nil
	}
	res := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		res = append(res, msg.Text)
	}
	return res
}

func listAdblockDomainsHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	bl := dnsData.GetBlockList()
	if bl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"domains": []string{}})
		return
	}
	domains := bl.GetAll()
	writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
}

func listAdblockSourcesHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	sources := dnsData.GetAdblockSources()
	if sources == nil {
		sources = []data.AdblockSource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func addAdblockDomainsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	var body struct {
		Domain  string   `json:"domain"`
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	dnsData := data.GetInstance()
	bl := dnsData.GetBlockList()
	if bl == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "adblock not available"})
		return
	}
	if body.Domain != "" {
		bl.AddDomain(strings.TrimSpace(body.Domain))
	}
	if len(body.Domains) > 0 {
		for _, d := range body.Domains {
			if strings.TrimSpace(d) != "" {
				bl.AddDomain(strings.TrimSpace(d))
			}
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "domains added", "count": bl.Count()})
}

func deleteAdblockDomainsHandler(w http.ResponseWriter, r *http.Request) {
	var domains []string
	if r.URL.RawQuery != "" {
		q := r.URL.Query()
		if d := q.Get("domain"); d != "" {
			domains = append(domains, d)
		}
		if d := q.Get("domains"); d != "" {
			for _, s := range strings.Split(d, ",") {
				domains = append(domains, strings.TrimSpace(s))
			}
		}
	}
	if r.Body != nil {
		var body struct {
			Domain  string   `json:"domain"`
			Domains []string `json:"domains"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.Domain != "" {
				domains = append(domains, body.Domain)
			}
			domains = append(domains, body.Domains...)
		}
	}
	dnsData := data.GetInstance()
	bl := dnsData.GetBlockList()
	if bl == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "adblock not available"})
		return
	}
	for _, d := range domains {
		if strings.TrimSpace(d) != "" {
			bl.RemoveDomain(strings.TrimSpace(d))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "domains removed", "count": bl.Count()})
}

func clearAdblockHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	bl := dnsData.GetBlockList()
	if bl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "adblock cleared"})
		return
	}
	bl.Clear()
	dnsData.ClearAdblockSources()
	writeJSON(w, http.StatusOK, map[string]any{"status": "adblock cleared"})
}

func getCacheHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	cache := dnsData.GetCacheRecords()
	writeJSON(w, http.StatusOK, map[string]any{"cache": cache})
}

func clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	dnsData.UpdateCacheRecords([]dnsrecordcache.CacheRecord{})
	writeJSON(w, http.StatusOK, map[string]any{"status": "cache cleared"})
}
