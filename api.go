package main

import (
	"dnsresolver/dnsrecords"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

// Wrapper for existing addRecord function
func addRecordGin(c *gin.Context) {
	// Read command from JSON request body
	var request []string
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	dnsrecords.AddRecord(request, gDNSRecords) // Call the existing addRecord function with parsed input
	c.JSON(201, gin.H{"status": "Record added"})
}

// Wrapper for existing listRecords function
func listRecordsGin(c *gin.Context) {
	// Call the existing listRecords function
	dnsrecords.ListRecords(gDNSRecords)
	c.JSON(200, gin.H{"status": "Listed"})
}

func startGinAPI(apiport string) {
	// Create a Gin router
	r := gin.Default()

	// Add routes for the API
	r.GET("/dns/records", listRecordsGin) // List all DNS records
	r.POST("/dns/records", addRecordGin)  // Add a new DNS record

	// Start the server
	if err := r.Run(fmt.Sprintf(":%s", apiport)); err != nil {
		log.Fatal("Error starting API:", err)
	}
}
