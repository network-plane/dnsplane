package main

import (
	"dnsresolver/data"
	"log"
	"os"
	"strings"
)

func createFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			log.Fatalf("Error creating %s: %s", filename, err)
		}
	}
}

// convertIPToReverseDNS takes an IP address and converts it to a reverse DNS lookup string.
func convertIPToReverseDNS(ip string) string {
	// Split the IP address into its segments
	parts := strings.Split(ip, ".")

	// Check if the input is a valid IPv4 address (should have exactly 4 parts)
	if len(parts) != 4 {
		return "Invalid IP address"
	}

	// Reverse the order of the IP segments
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	// Join the reversed segments and append the ".in-addr.arpa" domain
	reverseDNS := strings.Join(parts, ".") + ".in-addr.arpa"

	return reverseDNS
}

// convertReverseDNSToIP takes a reverse DNS lookup string and converts it back to an IP address.
func convertReverseDNSToIP(reverseDNS string) string {
	// Split the reverse DNS string by "."
	parts := strings.Split(reverseDNS, ".")

	// Check if the input is valid (should have at least 4 parts before "in-addr" and "arpa")
	if len(parts) < 6 {
		return "Invalid input"
	}

	// Extract the first four segments which represent the reversed IP address
	ipParts := parts[:4]

	// Reverse the order of the extracted segments
	for i, j := 0, len(ipParts)-1; i < j; i, j = i+1, j-1 {
		ipParts[i], ipParts[j] = ipParts[j], ipParts[i]
	}

	// Join the segments back together to form the original IP address
	return strings.Join(ipParts, ".")
}

func initializeJSONFiles() {
	createFileIfNotExists("dnsservers.json", `{"dnsservers":[{"address": "1.1.1.1","port": "53","active": false,"local_resolver": false,"adblocker": false }]}`)
	createFileIfNotExists("dnsrecords.json", `{"records": [{"name": "example.com.", "type": "A", "value": "93.184.216.34", "ttl": 3600, "last_query": "0001-01-01T00:00:00Z"}]}`)
	createFileIfNotExists("dnscache.json", `{"cache": [{"dns_record": {"name": "example.com","type": "A","value": "192.168.1.1","ttl": 3600,"added_on": "2024-05-01T12:00:00Z","updated_on": "2024-05-05T18:30:00Z","mac": "00:1A:2B:3C:4D:5E","last_query": "2024-05-07T15:45:00Z"},"expiry": "2024-05-10T12:00:00Z","timestamp": "2024-05-07T12:30:00Z","last_query": "2024-05-07T14:00:00Z"}]}`)
	createFileIfNotExists("dnsresolver.json", `{"fallback_server_ip": "192.168.178.21","fallback_server_port": "53","timeout": 2,"dns_port": "53","cache_records": true,"auto_build_ptr_from_a": true,"forward_ptr_queries": false,"file_locations": {"dnsserver_file": "dnsservers.json","dnsrecords_file": "dnsrecords.json","cache_file": "dnscache.json"}}`)
}

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
