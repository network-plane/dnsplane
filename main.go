package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/bettercap/readline"
	"github.com/gin-gonic/gin"
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

func startGinAPI(apiport string) {
	// Create a Gin router
	r := gin.Default()

	// Add routes for the API
	r.GET("/dns/records", listRecordsGin) // List all DNS records
	r.POST("/dns/records", addRecordGin)  // Add a new DNS record

	// Start the server
	if err := r.Run(fmt.Sprintf(":", apiport)); err != nil {
		log.Fatal("Error starting API:", err)
	}
}

func connectToUnixSocket(socketPath string) {
	conn, err := net.Dial("unix", socketPath) // Connect to the UNIX socket
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to UNIX socket: %v\n", err)
		return
	}
	defer conn.Close() // Ensure connection closure

	fmt.Println("Connected to UNIX socket:", socketPath)

	// Interactive mode setup from the given snippet
	config := readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/readline_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	}

	rl, err := readline.NewEx(&config) // Initialize readline
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return
	}
	defer rl.Close() // Close readline when done

	// Call the provided command handling loop
	handleCommandLoop(rl) // This handles user input in the interactive mode
}

func setupUnixSocketListener(socketPath string) {
	// Ensure there's no existing UNIX socket with the same name
	err := syscall.Unlink(socketPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal("Error removing existing UNIX socket:", err)
	}

	// Create the UNIX domain socket and listen for incoming connections
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("Error setting up UNIX socket listener:", err)
	}
	defer listener.Close()

	log.Printf("Listening on UNIX socket at %s", socketPath)

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Handle each connection in a separate goroutine
		go func(c net.Conn) {
			defer c.Close() // Ensure the connection is closed after processing

			buf := make([]byte, 1024) // Buffer for reading data from the connection
			n, err := c.Read(buf)     // Read the incoming data
			if err != nil {
				log.Printf("Error reading from connection: %v", err)
				return
			}

			command := string(buf[:n]) // Convert buffer to a string for command processing
			log.Printf("Received command: %s", command)

			// Add logic here to handle the received command, e.g., parsing and execution
			// For example, you could call a function to process the command:
			// handleCommand(command)
		}(conn) // Start the goroutine with the current connection
	}
}

func initializeJSONFiles() {
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
