package dnsserver

import "time"

// DNSServer holds the data for a DNS server
type DNSServer struct {
	Address       string    `json:"address"`
	Port          string    `json:"port"`
	Active        bool      `json:"active"`
	LocalResolver bool      `json:"local_resolver"`
	AdBlocker     bool      `json:"adblocker"`
	LastUsed      time.Time `json:"last_used,omitempty"`
	LastSuccess   time.Time `json:"last_success,omitempty"`
}
