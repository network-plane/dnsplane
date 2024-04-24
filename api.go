package main

import "github.com/gin-gonic/gin"

// Wrapper for existing addRecord function
func addRecordGin(c *gin.Context) {
	// Read command from JSON request body
	var request []string
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	addRecord(request) // Call the existing addRecord function with parsed input
	c.JSON(201, gin.H{"status": "Record added"})
}

// Wrapper for existing listRecords function
func listRecordsGin(c *gin.Context) {
	// Call the existing listRecords function
	listRecords()
	c.JSON(200, gin.H{"status": "Listed"})
}
