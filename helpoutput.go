package main

import "fmt"

func mainHelp() {
	fmt.Println("Available commands:")
	fmt.Printf("%-15s %s\n", "stats", "- Show server statistics")
	fmt.Printf("%-15s %s\n", "record", "- Record Management")
	fmt.Printf("%-15s %s\n", "cache", "- Cache Management")
	fmt.Printf("%-15s %s\n", "dns", "- DNS Server Management")
	fmt.Printf("%-15s %s\n", "server", "- Server Management")
	commonHelp()
}

func recordHelp() {
	fmt.Println("Record Management Sub Commands:")
	fmt.Printf("%-15s %s\n", "add", "- Add a new DNS record")
	fmt.Printf("%-15s %s\n", "remove", "- Remove a DNS record")
	fmt.Printf("%-15s %s\n", "update", "- Update a DNS record")
	fmt.Printf("%-15s %s\n", "list", "- List all DNS records")
	fmt.Printf("%-15s %s\n", "clear", "- Clear all DNS records")
	fmt.Printf("%-15s %s\n", "test", "- Test a DNS record")
	fmt.Printf("%-15s %s\n", "load", "- Load DNS records from a file")
	fmt.Printf("%-15s %s\n", "save", "- Save DNS records to a file")
	commonHelp()
}

func cacheHelp() {
	fmt.Println("Cache Management Sub Commands:")
	fmt.Printf("%-15s %s\n", "clear", "- Clear the cache")
	fmt.Printf("%-15s %s\n", "list", "- List all cache entries")
	commonHelp()
}

func dnsHelp() {
	fmt.Println("DNS Server Management Sub Commands:")
	fmt.Printf("%-15s %s\n", "add", "- Add a new DNS server")
	fmt.Printf("%-15s %s\n", "remove", "- Remove a DNS server")
	fmt.Printf("%-15s %s\n", "update", "- Update a DNS server")
	fmt.Printf("%-15s %s\n", "list", "- List all DNS servers")
	fmt.Printf("%-15s %s\n", "clear", "- Clear all DNS servers")
	fmt.Printf("%-15s %s\n", "test", "- Test a DNS server")
	fmt.Printf("%-15s %s\n", "load", "- Load DNS servers from a file")
	fmt.Printf("%-15s %s\n", "save", "- Save DNS servers to a file")
	commonHelp()
}

func serverHelp() {
	fmt.Println("dnsresolve Management Sub Commands:")
	fmt.Printf("%-15s %s\n", "start", "- Start the server")
	fmt.Printf("%-15s %s\n", "stop", "- Stop the server")
	fmt.Printf("%-15s %s\n", "restart", "- Restart the server")
	fmt.Printf("%-15s %s\n", "status", "- Show server status")
	fmt.Printf("%-15s %s\n", "fallback", "- Set/List the fallback server")
	fmt.Printf("%-15s %s\n", "timeout", "- Set/List the server timeout")
	fmt.Printf("%-15s %s\n", "save", "- Save the current settings")
	fmt.Printf("%-15s %s\n", "load", "- Load the settings from the files")
	commonHelp()
}

func commonHelp() {
	fmt.Printf("%-15s %s\n", "/", "- Go up one level")
	fmt.Printf("%-15s %s\n", "exit, quit, q", "- Shutdown the server")
	fmt.Printf("%-15s %s\n", "help, h, ?", "- Show help")
}
