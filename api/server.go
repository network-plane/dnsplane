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

	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnsrecords"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	apiServerMu sync.Mutex
	apiServer   *http.Server
)

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
	router.Get("/dns/records", listRecordsHandler)
	router.Post("/dns/records", addRecordHandler)
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
