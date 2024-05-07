package dnsserver

import (
	"fmt"
	"net"
	"strconv"
)

// GetDNSArray returns an array of DNS servers in the format "Address:Port".
func GetDNSArray(dnsServers []DNSServer, activeonly bool) []string {
	var dnsArray []string
	for _, dnsServer := range dnsServers {
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
		fmt.Println("Enter the DNS Server in the format: Address Port Active Local AdBlocker")
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
func Remove(fullCommand []string, dnsServers []DNSServer) []DNSServer {
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

	for i, dnsServer := range dnsServers {
		if dnsServer.Address == address {
			fmt.Println("Removed: ", address)
			return append(dnsServers[:i], dnsServers[i+1:]...)
		}
	}

	fmt.Println("No DNS server found with the address:", address)
	return dnsServers
}

// List lists all the DNS servers in the list of DNS servers.
func List(dnsServers []DNSServer) {
	if len(dnsServers) == 0 {
		fmt.Println("No DNS servers found.")
		return
	}

	fmt.Printf("%-20s %-5s %-6s %-6s %-9s\n", "Address", "Port", "Active", "Local", "AdBlocker")
	for _, dnsServer := range dnsServers {
		fmt.Printf("%-20s %-5s %-6t %-6t %-9t\n", dnsServer.Address, dnsServer.Port, dnsServer.Active, dnsServer.LocalResolver, dnsServer.AdBlocker)
	}
}
