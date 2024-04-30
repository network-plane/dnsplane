package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bettercap/readline"
	cli "github.com/jawher/mow.cli"
	"github.com/miekg/dns"
)

func main() {
	app := cli.App("dnsapp", "DNS Server with optional CLI mode")
	app.Version("v version", fmt.Sprintf("DNS Resolver %s", appversion))

	// Command-line options
	daemon := app.BoolOpt("d daemon", false, "Run as daemon (no interactive mode)")
	port := app.StringOpt("p port", "53", "Port for DNS server")
	remoteUnix := app.StringOpt("u remote-unix", "/tmp/dnsresolver.socket", "Path to UNIX domain socket")
	clientMode := app.BoolOpt("c client-mode", false, "Run in client mode (connect to UNIX socket)")
	apiMode := app.BoolOpt("a api", false, "Enable the REST API")
	apiport := app.StringOpt("apiport", "8080", "Port for the REST API")

	app.Action = func() {
		if *clientMode && *remoteUnix != "" {
			connectToUnixSocket(*remoteUnix) // Connect to UNIX socket as client
			return
		}

		if *apiMode {
			go startGinAPI(*apiport) // Start the REST API
		}

		initializeJSONFiles()

		dns.HandleFunc(".", handleRequest)

		dnsServerSettings = getSettings()
		server := &dns.Server{
			Addr: fmt.Sprintf(":%s", *port),
			Net:  "udp",
		}

		go func() {
			log.Printf("Starting DNS server on %s\n", server.Addr)
			dnsStats.ServerStartTime = time.Now()
			if err := server.ListenAndServe(); err != nil {
				fmt.Println("Error starting server:", err)
				os.Exit(1)
			}
		}()

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
				Prompt:          "> ",
				HistoryFile:     "/tmp/dnsresolver.history",
				InterruptPrompt: "^C",
				EOFPrompt:       "exit",
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

	if _, err := os.Stat("records.json"); os.IsExist(err) {
		dnsRecords = getDNSRecords()
	}

	fmt.Println("records loaded: ", len(dnsRecords))

	createFileIfNotExists("servers.json", `{"servers": ["1.1.1.1:53", "1.0.0.1:53"]}`)
	createFileIfNotExists("records.json", `{"records": [{"Name": "example.com.", "Type": "A", "Value": "93.184.216.34", "TTL": 3600}]}`)
	createFileIfNotExists("cache.json", `{"records": []}`)
	createFileIfNotExists("dnsresolver.json", `{"fallback_server_ip": "192.168.178.21", "fallback_server_port": "53", "timeout": 2, "dns_port": "53", "cache_records": true, "auto_build_ptr_from_a": true}`)
}

func createFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			log.Fatalf("Error creating %s: %s", filename, err)
		}
	}
}
