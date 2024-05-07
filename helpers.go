package main

import (
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
