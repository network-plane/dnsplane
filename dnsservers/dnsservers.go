// Package dnsservers provides the data structure and functions for managing DNS servers.
package dnsservers

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

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

// GetDNSArray returns an array of DNS servers in the format "Address:Port".
func GetDNSArray(dnsServerData []DNSServer, activeOnly bool) []string {
	var dnsArray []string
	for _, dnsServer := range dnsServerData {
		if activeOnly && !dnsServer.Active {
			continue
		}
		dnsArray = append(dnsArray, dnsServer.Address+":"+dnsServer.Port)
	}
	return dnsArray
}

// Add adds a DNS server to the list of DNS servers.
func Add(fullCommand []string, dnsServers []DNSServer) []DNSServer {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage: add <Address> [Port] [Active] [LocalResolver] [AdBlocker]")
		fmt.Println("Example: add 1.1.1.1 53 true false false")
		return dnsServers
	}

	server := DNSServer{
		Port:          "53",
		Active:        true,
		LocalResolver: true,
		AdBlocker:     false,
	}

	if err := applyArgsToDNSServer(&server, fullCommand); err != nil {
		fmt.Println("Error:", err)
		return dnsServers
	}

	return append(dnsServers, server)
}

// Remove removes a DNS server from the list of DNS servers.
func Remove(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage: remove <Address>")
		fmt.Println("Example: remove 127.0.0.1")
		return dnsServerData
	}

	if len(fullCommand) < 2 {
		fmt.Println("Error: Address is required.")
		return dnsServerData
	}

	address := fullCommand[1]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		fmt.Println("Error: No DNS server found with the address:", address)
		return dnsServerData
	}

	fmt.Println("Removed DNS server:", address)
	return append(dnsServerData[:index], dnsServerData[index+1:]...)
}

// Update modifies a DNS server's record in the list of DNS servers.
func Update(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage: update <Address> [Port] [Active] [LocalResolver] [AdBlocker]")
		fmt.Println("Example: update 1.1.1.1 53 false true true")
		return dnsServerData
	}

	if len(fullCommand) < 2 {
		fmt.Println("Error: Address is required.")
		return dnsServerData
	}

	address := fullCommand[1]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		fmt.Println("Error: DNS server not found:", address)
		return dnsServerData
	}

	server := dnsServerData[index]
	if err := applyArgsToDNSServer(&server, fullCommand); err != nil {
		fmt.Println("Error:", err)
		return dnsServerData
	}

	dnsServerData[index] = server
	return dnsServerData
}

// List lists all the DNS servers in the list of DNS servers.
func List(dnsServerData []DNSServer) {
	if len(dnsServerData) == 0 {
		fmt.Println("No DNS servers found.")
		return
	}

	fmt.Printf("%-20s %-5s %-6s %-6s %-9s\n", "Address", "Port", "Active", "Local", "AdBlocker")
	for _, dnsServer := range dnsServerData {
		fmt.Printf("%-20s %-5s %-6t %-6t %-9t\n", dnsServer.Address, dnsServer.Port, dnsServer.Active, dnsServer.LocalResolver, dnsServer.AdBlocker)
	}
}

// Helper function to parse and apply command arguments to a DNSServer.
func applyArgsToDNSServer(server *DNSServer, args []string) error {
	if len(args) >= 2 {
		server.Address = args[1]
		if net.ParseIP(server.Address) == nil {
			return fmt.Errorf("invalid IP address: %s", server.Address)
		}
	} else {
		return fmt.Errorf("address is required")
	}

	if len(args) >= 3 {
		if _, err := strconv.Atoi(args[2]); err != nil {
			return fmt.Errorf("invalid port: %s", args[2])
		}
		server.Port = args[2]
	}

	boolFields := []struct {
		name     string
		index    int
		assignTo *bool
	}{
		{"Active", 3, &server.Active},
		{"LocalResolver", 4, &server.LocalResolver},
		{"AdBlocker", 5, &server.AdBlocker},
	}

	for _, field := range boolFields {
		if len(args) > field.index {
			value, err := strconv.ParseBool(args[field.index])
			if err != nil {
				return fmt.Errorf("invalid value for %s: %s", field.name, args[field.index])
			}
			*field.assignTo = value
		}
	}

	return nil
}

// Helper function to find the index of a DNSServer by address.
func findDNSServerIndex(dnsServers []DNSServer, address string) int {
	for i, server := range dnsServers {
		if server.Address == address {
			return i
		}
	}
	return -1
}

// Helper function to handle the help command.
func checkHelpCommand(fullCommand []string) bool {
	return len(fullCommand) > 1 && fullCommand[1] == "?"
}
