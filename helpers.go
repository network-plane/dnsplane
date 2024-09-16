package main

import (
	"dnsresolver/data"
	"log"
)

// loadSettings reads the dnsresolver.json file and returns the DNS server settings
func loadSettings() DNSResolverSettings {
	return data.LoadFromJSON[DNSResolverSettings]("dnsresolver.json")
}

// saveSettings saves the DNS server settings to the dnsresolver.json file
func saveSettings(settings DNSResolverSettings) {
	if err := data.SaveToJSON("dnsresolver.json", settings); err != nil {
		log.Fatalf("Failed to save settings: %v", err)
	}
}
