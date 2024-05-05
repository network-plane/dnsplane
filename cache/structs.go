package cache

import (
	"dnsresolver/dnsrecords"
	"time"
)

// Record holds the data for the cache records
type Record struct {
	DNSRecord dnsrecords.DNSRecord
	Expiry    time.Time
	Timestamp time.Time
	LastQuery time.Time
}
