package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnsrecords"
	"dnsplane/fullstats"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	apiServerMu         sync.Mutex
	apiServer           *http.Server
	apiState            *daemon.State
	apiFullStatsTracker *fullstats.Tracker
)

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

// Start launches the REST API server asynchronously. If registrar is nil, the
// package's default DNS routes are registered. The server listens on the
// provided port and updates the daemon state when it stops.
// If logger is nil, no file logging is done for API messages.
func Start(state *daemon.State, port string, registrar RouteRegistrar, logger *slog.Logger) {
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

	apiServerMu.Lock()
	apiState = state
	apiServerMu.Unlock()
	state.SetAPIRunning(true)
	if logger != nil {
		logger.Info("API server starting", "port", trimmed)
	}
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	registrar(router)

	addr := fmt.Sprintf(":%s", trimmed)
	srv := &http.Server{Addr: addr, Handler: router}
	apiServerMu.Lock()
	apiServer = srv
	apiServerMu.Unlock()
	go func() {
		defer state.SetAPIRunning(false)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	router.Get("/dns/records", listRecordsHandler)
	router.Post("/dns/records", addRecordHandler)
	router.Get("/dns/servers", listServersHandler)
	router.Get("/stats", statsHandler)
	router.Get("/metrics", metricsHandler)
	router.Get("/stats/page", statsPageHandler)
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

// listServersHandler returns the list of configured upstream DNS servers (read-only).
func listServersHandler(w http.ResponseWriter, r *http.Request) {
	dnsData := data.GetInstance()
	servers := dnsData.GetServers()
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}

// resolverStatsMap returns the resolver stats as a map for session/total (both same until persistence exists).
func resolverStatsMap(stats data.DNSStats) map[string]any {
	return map[string]any{
		"total_queries":            stats.TotalQueries,
		"total_cache_hits":         stats.TotalCacheHits,
		"total_blocks":             stats.TotalBlocks,
		"total_queries_forwarded":  stats.TotalQueriesForwarded,
		"total_queries_answered":   stats.TotalQueriesAnswered,
		"server_start_time":        stats.ServerStartTime.Format(time.RFC3339),
	}
}

// statsHandler returns resolver and full_stats as JSON with session and total scope.
// Resolver stats are in-memory only (since server start), so session and total are the same for now.
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

	writeJSON(w, http.StatusOK, map[string]any{"session": sessionMap, "total": totalMap})
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
		fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
		fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typeName)
		switch v := m.value.(type) {
		case int:
			fmt.Fprintf(w, "%s %d\n", m.name, v)
		case float64:
			fmt.Fprintf(w, "%s %.0f\n", m.name, v)
		default:
			fmt.Fprintf(w, "%s %v\n", m.name, v)
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
			fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
			fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typeName)
			fmt.Fprintf(w, "%s %d\n", m.name, m.value.(int))
		}
	}
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
	dnsData.UpdateRecords(updated)
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
