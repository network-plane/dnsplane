package api

import (
	"errors"
	"fmt"
	"log"
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
func Start(state *daemon.State, port string, registrar RouteRegistrar) {
	if state == nil {
		log.Printf("api: missing daemon state; cannot start API")
		return
	}
	trimmed := strings.TrimSpace(port)
	if trimmed == "" {
		log.Printf("api: invalid port; refusing to start")
		return
	}
	if state.APIRunning() {
		log.Printf("api: server already running, skipping start")
		return
	}
	if registrar == nil {
		registrar = RegisterDNSRoutes
	}

	state.SetAPIRunning(true)
	go func() {
		defer state.SetAPIRunning(false)

		router := gin.Default()
		registrar(router)

		if err := router.Run(fmt.Sprintf(":%s", trimmed)); err != nil {
			log.Printf("api: server stopped with error: %v", err)
		}
	}()
}

// RegisterDNSRoutes wires up the default DNS-related REST handlers.
func RegisterDNSRoutes(router *gin.Engine) {
	if router == nil {
		return
	}
	router.GET("/dns/records", listRecordsHandler)
	router.POST("/dns/records", addRecordHandler)
}

func addRecordHandler(c *gin.Context) {
	dnsData := data.GetInstance()
	var request []string
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	updated, messages, err := dnsrecords.Add(request, dnsData.DNSRecords, false)
	if errors.Is(err, dnsrecords.ErrHelpRequested) {
		c.JSON(200, gin.H{"messages": extractRecordMessages(messages)})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error(), "messages": extractRecordMessages(messages)})
		return
	}
	dnsData.UpdateRecords(updated)
	c.JSON(201, gin.H{"status": "record added", "messages": extractRecordMessages(messages)})
}

func listRecordsHandler(c *gin.Context) {
	dnsData := data.GetInstance()
	result, err := dnsrecords.List(dnsData.DNSRecords, []string{})
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
