package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnsrecords"

	"github.com/gin-gonic/gin"
)

// RouteRegistrar registers HTTP routes on the supplied Gin engine.
type RouteRegistrar func(*gin.Engine)

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
	go func() {
		defer state.SetAPIRunning(false)

		router := gin.Default()
		registrar(router)

		if err := router.Run(fmt.Sprintf(":%s", trimmed)); err != nil {
			logAPIError(logger, "API server stopped with error", "error", err)
		}
	}()
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
func RegisterDNSRoutes(router *gin.Engine) {
	if router == nil {
		return
	}
	router.GET("/dns/records", listRecordsHandler)
	router.POST("/dns/records", addRecordHandler)
}

type AddRecordRequest struct {
	Name  string  `json:"name" binding:"required"`
	Type  string  `json:"type" binding:"required"`
	Value string  `json:"value" binding:"required"`
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

func addRecordHandler(c *gin.Context) {
	dnsData := data.GetInstance()
	var request AddRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
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
		c.JSON(status, gin.H{"error": err.Error(), "messages": extractRecordMessages(messages)})
		return
	}
	dnsData.UpdateRecords(updated)
	c.JSON(http.StatusCreated, gin.H{"status": "record added", "messages": extractRecordMessages(messages)})
}

func listRecordsHandler(c *gin.Context) {
	dnsData := data.GetInstance()
	records := dnsData.GetRecords()
	result, err := dnsrecords.List(records, []string{})
	if errors.Is(err, dnsrecords.ErrHelpRequested) {
		c.JSON(200, gin.H{"messages": extractRecordMessages(result.Messages)})
		return
	}
	resp := gin.H{
		"records":  result.Records,
		"detailed": result.Detailed,
	}
	if result.Filter != "" {
		resp["filter"] = result.Filter
	}
	if len(result.Messages) > 0 {
		resp["messages"] = extractRecordMessages(result.Messages)
	}
	c.JSON(200, resp)
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
