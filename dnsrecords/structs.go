package dnsrecords

import "time"

// DNSRecord holds the data for a DNS record
type DNSRecord struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Value      string    `json:"value"`
	TTL        uint32    `json:"ttl"`
	AddedOn    time.Time `json:"added_on,omitempty"`
	UpdatedOn  time.Time `json:"updated_on,omitempty"`
	MACAddress string    `json:"mac,omitempty"`
	LastQuery  time.Time `json:"last_query,omitempty"`
}
