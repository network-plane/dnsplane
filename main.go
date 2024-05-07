// Description: Main entry point for the DNS Resolver application.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"dnsresolver/data"

	// "github.com/bettercap/readline"
	"github.com/chzyer/readline"
	cli "github.com/jawher/mow.cli"
	"github.com/miekg/dns"
)

func main() {
	app := cli.App("dnsapp", "DNS Server with optional CLI mode")
	app.Version("v version", fmt.Sprintf("DNS Resolver %s", appversion))

	// Command-line options
	daemon := app.BoolOpt("daemon", false, "Run as daemon (no interactive mode)")
	port := app.StringOpt("port", "53", "Port for DNS server")
	remoteUnix := app.StringOpt("remote-unix", "/tmp/dnsresolver.socket", "Path to UNIX domain socket")
	clientMode := app.BoolOpt("client-mode", false, "Run in client mode (connect to UNIX socket)")
	apiMode := app.BoolOpt("api", false, "Enable the REST API")
	apiport := app.StringOpt("apiport", "8080", "Port for the REST API")
	mdnsMode := app.BoolOpt("mdns", false, "Enable mDNS server")
	mdnsPort := app.StringOpt("mdns-port", "5353", "Port for mDNS server")

	app.Action = func() {
		//if we run in client mode we dont need to run the rest of the code
		if *clientMode && *remoteUnix != "" {
			connectToUnixSocket(*remoteUnix) // Connect to UNIX socket as client
			return
		}

		// Start the REST API if enabled
		if *apiMode {
			go startGinAPI(*apiport)
		}

		//Create JSON files if they don't exist
		initializeJSONFiles()

		//Load Data
		gDNSRecords = data.LoadDNSRecords()
		dnsServerSettings = loadSettings()
		cacheRecords = data.LoadCacheRecords()
		dnsServers = data.LoadDNSServers()

		// Set up the DNS server handler
		dns.HandleFunc(".", handleRequest)

		// Configure the DNS server settings
		server := &dns.Server{
			Addr: fmt.Sprintf(":%s", *port),
			Net:  "udp",
		}

		// Start the DNS server
		go func() {
			log.Printf("Starting DNS server on %s\n", server.Addr)
			dnsStats.ServerStartTime = time.Now()
			if err := server.ListenAndServe(); err != nil {
				fmt.Println("Error starting server:", err)
				os.Exit(1)
			}
		}()

		// mDNS server setup
		if *mdnsMode {
			go startMDNSServer(*mdnsPort)
		}

		// If running in daemon mode, exit after starting the server
		if *daemon {
			if *remoteUnix != "" {
				setupUnixSocketListener(*remoteUnix) // Set up UNIX socket listener for daemon mode
			} else {
				select {} // Keeps the program alive
			}
		} else {
			// Interactive Mode
			config := readline.Config{
				Prompt:            "> ",
				HistoryFile:       "/tmp/dnsresolver.history",
				InterruptPrompt:   "^C",
				EOFPrompt:         "exit",
				HistorySearchFold: true,
			}

			rl, err := readline.NewEx(&config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "readline: %v\n", err)
				return
			}
			defer rl.Close()

			handleCommandLoop(rl) // Call the function for command handling
		}
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func initializeJSONFiles() {
	createFileIfNotExists("servers.json", `{"servers": ["1.1.1.1:53", "1.0.0.1:53"]}`)
	createFileIfNotExists("records.json", `{"records": [{"name": "example.com.", "type": "A", "value": "93.184.216.34", "ttl": 3600, "last_query": "0001-01-01T00:00:00Z"}]}`)
	createFileIfNotExists("cache.json", `{"cache": [{"name": "example.com.","type": "A","value": "127.0.0.1","ttl": 300,"last_query": "0001-01-01T00:00:00Z"}]}`)
	createFileIfNotExists("dnsresolver.json", `{"fallback_server_ip": "192.168.178.21", "fallback_server_port": "53", "timeout": 2, "dns_port": "53", "cache_records": true, "auto_build_ptr_from_a": true}`)
}

// loadSettings reads the dnsresolver.json file and returns the DNS server settings
func loadSettings() DNSServerSettings {
	return data.LoadFromJSON[DNSServerSettings]("dnsresolver.json")
}

// saveSettings saves the DNS server settings to the dnsresolver.json file
func saveSettings(settings DNSServerSettings) {
	if err := data.SaveToJSON("dnsresolver.json", settings); err != nil {
		log.Fatalf("Failed to save settings: %v", err)
	}
}
