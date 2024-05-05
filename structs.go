package main

import (
	"dnsresolver/dnsrecords"
	"time"
)

var (
	dnsServerSettings DNSServerSettings
	dnsServers        []string
	dnsStats          DNSStats
	gDNSRecords       []dnsrecords.DNSRecord
	cacheRecords      []CacheRecord
	appversion        = "0.1.2"
)

// DNSStats holds the data for the DNS statistics
type DNSStats struct {
	TotalQueries          int `json:"total_queries"`
	TotalCacheHits        int `json:"total_cache_hits"`
	TotalBlocks           int `json:"total_blocks"`
	TotalQueriesForwarded int `json:"total_queries_forwarded"`
	TotalQueriesAnswered  int `json:"total_queries_answered"`
	ServerStartTime       time.Time
}

// DNSServerSettings holds DNS server settings
type DNSServerSettings struct {
	FallbackServerIP   string `json:"fallback_server_ip"`
	FallbackServerPort string `json:"fallback_server_port"`
	Timeout            int    `json:"timeout"`
	DNSPort            string `json:"dns_port"`
	CacheRecords       bool   `json:"cache_records"`
	AutoBuildPTRFromA  bool   `json:"auto_build_ptr_from_a"`
	ForwardPTRQueries  bool   `json:"forward_ptr_queries"`
}

// Servers holds the data for the servers
type Servers struct {
	Servers []string `json:"servers"`
}

// CacheRecord holds the data for the cache records
type CacheRecord struct {
	DNSRecord dnsrecords.DNSRecord
	Expiry    time.Time
	Timestamp time.Time
	LastQuery time.Time
}
