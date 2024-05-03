package main

import (
	"fmt"
	"time"
)

// serverUpTimeFormat formats the time duration since the server start time into a human-readable string.
func serverUpTimeFormat(startTime time.Time) string {
	duration := time.Since(startTime)

	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes, %d seconds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes, %d seconds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func showStats() {
	fmt.Println("Stats:")
	fmt.Println("Server start time:", dnsStats.ServerStartTime)
	fmt.Println("Server Up Time:", serverUpTimeFormat(dnsStats.ServerStartTime))
	fmt.Println()
	fmt.Println("Total Records:", len(dnsRecords))
	fmt.Println("Total DNS Servers:", len(loadDNSServers()))
	fmt.Println("Total Cache Records:", len(cacheRecords))
	fmt.Println()
	fmt.Println("Total queries received:", dnsStats.TotalQueries)
	fmt.Println("Total queries answered:", dnsStats.TotalQueriesAnswered)
	fmt.Println("Total cache hits:", dnsStats.TotalCacheHits)
	fmt.Println("Total queries forwarded:", dnsStats.TotalQueriesForwarded)
}
