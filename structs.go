package main

import "time"

var (
	dnsServerSettings DNSServerSettings
	dnsStats          DNSStats
	dnsRecords        []DNSRecord
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

// DNSRecord holds the data for a DNS record
type DNSRecord struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	TTL       uint32    `json:"ttl"`
	LastQuery time.Time `json:"last_query"`
}

// Servers holds the data for the servers
type Servers struct {
	Servers []string `json:"servers"`
}

// Records holds the data for the DNS records
// type Records struct {
// 	Records []DNSRecord `json:"records"`
// }

// CacheRecord holds the data for the cache records
type CacheRecord struct {
	DNSRecord DNSRecord
	Expiry    time.Time
	Timestamp time.Time
	LastQuery time.Time
}
