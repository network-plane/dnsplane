package cache

import (
	"dnsresolver/dnsrecords"
	"time"
)

// Record holds the data for the cache records
type Record struct {
	DNSRecord dnsrecords.DNSRecord `json:"dns_record"`
	Expiry    time.Time            `json:"expiry,omitempty"`
	Timestamp time.Time            `json:"timestamp,omitempty"`
	LastQuery time.Time            `json:"last_query,omitempty"`
}
