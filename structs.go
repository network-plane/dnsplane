package main

import "time"

type DnsRecord struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	TTL       uint32    `json:"ttl"`
	LastQuery time.Time `json:"last_query"`
}

type Servers struct {
	Servers []string `json:"servers"`
}

type Records struct {
	Records []DnsRecord `json:"records"`
}

type CacheRecord struct {
	DnsRecord DnsRecord
	Expiry    time.Time
	Timestamp time.Time
	LastQuery time.Time
}
