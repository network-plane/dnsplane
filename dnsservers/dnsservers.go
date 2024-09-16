// Package dnsserver provides the data structure and functions for managing DNS servers.
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
func GetDNSArray(dnsServerData []DNSServer, activeonly bool) []string {
	var dnsArray []string
	for _, dnsServer := range dnsServerData {
		if activeonly && !dnsServer.Active {
			continue
		}
		dnsArray = append(dnsArray, dnsServer.Address+":"+dnsServer.Port)
	}
	return dnsArray
}

// Add adds a DNS server to the list of DNS servers.
func Add(fullCommand []string, dnsServers []DNSServer) []DNSServer {
	const defaultPort = "53"

	// Default DNS server settings
	server := DNSServer{
		Port:          defaultPort,
		Active:        true,
		LocalResolver: true,
		AdBlocker:     false,
	}

	if len(fullCommand) > 1 && fullCommand[1] == "?" {
		fmt.Println("Enter the DNS Server in the format: <Address> [Port] [Active] [Local] [AdBlocker]")
		fmt.Println("Example: 1.1.1.1 53 true false false")
		return nil
	}

	if len(fullCommand) >= 2 {
		server.Address = fullCommand[1]
		if net.ParseIP(server.Address) == nil {
			fmt.Println("Invalid IP address:", server.Address)
			return nil
		}
	}

	if len(fullCommand) >= 3 {
		if _, err := strconv.Atoi(fullCommand[2]); err != nil {
			fmt.Println("Invalid port:", fullCommand[2])
			return nil
		}
		server.Port = fullCommand[2]
	}

	if len(fullCommand) >= 5 {
		if active, err := strconv.ParseBool(fullCommand[3]); err == nil {
			server.Active = active
		}
		if localResolver, err := strconv.ParseBool(fullCommand[4]); err == nil {
			server.LocalResolver = localResolver
		}
		if adBlocker, err := strconv.ParseBool(fullCommand[5]); err == nil {
			server.AdBlocker = adBlocker
		}
	}

	return append(dnsServers, server)
}

// Remove removes a DNS server from the list of DNS servers.
func Remove(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if len(fullCommand) > 1 && fullCommand[1] == "?" {
		fmt.Println("Enter the DNS server address to remove.")
		fmt.Println("Example: 127.0.0.1")
		return nil
	}

	if len(fullCommand) < 2 {
		fmt.Println("Invalid DNS server address.")
		return nil
	}

	address := fullCommand[1]

	for i, dnsServer := range dnsServerData {
		if dnsServer.Address == address {
			fmt.Println("Removed: ", address)
			return append(dnsServerData[:i], dnsServerData[i+1:]...)
		}
	}

	fmt.Println("No DNS server found with the address:", address)
	return dnsServerData
}

// Update modifies a DNS server's record in the list of DNS servers.
func Update(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if len(fullCommand) < 2 {
		fmt.Println("Usage: update <Address> [Port] [Active] [Local] [AdBlocker]")
		return dnsServerData
	}

	address := fullCommand[1]
	serverIndex := -1

	// Find the server in the existing list
	for i, server := range dnsServerData {
		if server.Address == address {
			serverIndex = i
			break
		}
	}

	if serverIndex == -1 {
		fmt.Println("DNS server not found:", address)
		return dnsServerData
	}

	server := dnsServerData[serverIndex]

	if len(fullCommand) >= 3 {
		if _, err := strconv.Atoi(fullCommand[2]); err != nil {
			fmt.Println("Invalid port:", fullCommand[2])
			return dnsServerData
		}
		server.Port = fullCommand[2]
	}

	if len(fullCommand) >= 4 {
		if active, err := strconv.ParseBool(fullCommand[3]); err == nil {
			server.Active = active
		} else {
			fmt.Println("Invalid value for Active:", fullCommand[3])
			return dnsServerData
		}
	}

	if len(fullCommand) >= 5 {
		if localResolver, err := strconv.ParseBool(fullCommand[4]); err == nil {
			server.LocalResolver = localResolver
		} else {
			fmt.Println("Invalid value for Local Resolver:", fullCommand[4])
			return dnsServerData
		}
	}

	if len(fullCommand) >= 6 {
		if adBlocker, err := strconv.ParseBool(fullCommand[5]); err == nil {
			server.AdBlocker = adBlocker
		} else {
			fmt.Println("Invalid value for AdBlocker:", fullCommand[5])
			return dnsServerData
		}
	}

	dnsServerData[serverIndex] = server
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
