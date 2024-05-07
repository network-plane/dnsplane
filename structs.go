package main

import (
	"dnsresolver/cache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
	"time"
)

var (
	dnsServerSettings DNSResolverSettings
	dnsServers        []dnsserver.DNSServer
	dnsStats          DNSStats
	gDNSRecords       []dnsrecords.DNSRecord
	cacheRecords      []cache.Record

	appversion = "0.1.2"
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

// DNSResolverSettings holds DNS server settings
type DNSResolverSettings struct {
	FallbackServerIP   string        `json:"fallback_server_ip"`
	FallbackServerPort string        `json:"fallback_server_port"`
	Timeout            int           `json:"timeout"`
	DNSPort            string        `json:"dns_port"`
	CacheRecords       bool          `json:"cache_records"`
	AutoBuildPTRFromA  bool          `json:"auto_build_ptr_from_a"`
	ForwardPTRQueries  bool          `json:"forward_ptr_queries"`
	FileLocations      fileLocations `json:"file_locations"`
}

type fileLocations struct {
	DNSServerFile  string `json:"dnsserver_file"`
	DNSRecordsFile string `json:"dnsrecords_file"`
	CacheFile      string `json:"cache_file"`
}

// Servers holds the data for the servers
type Servers struct {
	Servers []string `json:"servers"`
}

type cmdHelp struct {
	Name        string
	Description string
	SubCommands map[string]cmdHelp
}
