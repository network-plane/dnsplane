// Package dnsservers provides the data structure and functions for managing DNS servers.
package dnsservers

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"dnsresolver/cliutil"
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
	if cliutil.IsHelpRequest(fullCommand) {
		printAddUsage()
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
		printAddUsage()
		return dnsServers
	}

	return append(dnsServers, server)
}

// Remove removes a DNS server from the list of DNS servers.

func Remove(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if cliutil.IsHelpRequest(fullCommand) {
		printRemoveUsage()
		return dnsServerData
	}

	if len(fullCommand) < 1 {
		fmt.Println("Error: address is required.")
		printRemoveUsage()
		return dnsServerData
	}
	if len(fullCommand) > 1 {
		fmt.Println("Error: provide only the server address.")
		printRemoveUsage()
		return dnsServerData
	}

	address := fullCommand[0]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		fmt.Println("Error: No DNS server found with the address:", address)
		printRemoveUsage()
		return dnsServerData
	}

	fmt.Println("Removed DNS server:", address)
	return append(dnsServerData[:index], dnsServerData[index+1:]...)
}

// Update modifies a DNS server's record in the list of DNS servers.

func Update(fullCommand []string, dnsServerData []DNSServer) []DNSServer {
	if cliutil.IsHelpRequest(fullCommand) {
		printUpdateUsage()
		return dnsServerData
	}

	if len(fullCommand) < 1 {
		fmt.Println("Error: address is required.")
		printUpdateUsage()
		return dnsServerData
	}

	address := fullCommand[0]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		fmt.Println("Error: DNS server not found:", address)
		printUpdateUsage()
		return dnsServerData
	}

	server := dnsServerData[index]
	if err := applyArgsToDNSServer(&server, fullCommand); err != nil {
		fmt.Println("Error:", err)
		printUpdateUsage()
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
	if len(args) >= 1 {
		server.Address = args[0]
		if net.ParseIP(server.Address) == nil {
			return fmt.Errorf("invalid IP address: %s", server.Address)
		}
	} else {
		return fmt.Errorf("address is required")
	}

	if len(args) >= 2 {
		if _, err := strconv.Atoi(args[1]); err != nil {
			return fmt.Errorf("invalid port: %s", args[1])
		}
		server.Port = args[1]
	}

	boolFields := []struct {
		name     string
		index    int
		assignTo *bool
	}{
		{"Active", 2, &server.Active},
		{"LocalResolver", 3, &server.LocalResolver},
		{"AdBlocker", 4, &server.AdBlocker},
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

	maxArgs := 2 + len(boolFields)
	if len(args) > maxArgs {
		return fmt.Errorf("too many arguments provided; expected at most %d parameters", maxArgs)
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
func printAddUsage() {
	fmt.Println("Usage  : add <Address> [Port] [Active] [LocalResolver] [AdBlocker]")
	fmt.Println("Example: add 1.1.1.1 53 true false false")
	printHelpAliasesHint()
}

func printRemoveUsage() {
	fmt.Println("Usage  : remove <Address>")
	fmt.Println("Example: remove 127.0.0.1")
	printHelpAliasesHint()
}

func printUpdateUsage() {
	fmt.Println("Usage  : update <Address> [Port] [Active] [LocalResolver] [AdBlocker]")
	fmt.Println("Example: update 1.1.1.1 53 false true true")
	printHelpAliasesHint()
}

func printHelpAliasesHint() {
	fmt.Println("Hint: append '?', 'help', or 'h' after the command to view this usage.")
}
