// Description: Main entry point for the DNS Resolver application.
package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"

	// "github.com/bettercap/readline"
	"github.com/chzyer/readline"
	cli "github.com/jawher/mow.cli"
	"github.com/miekg/dns"
)

var (
	dnsServerSettings DNSResolverSettings
	dnsServers        []dnsserver.DNSServer
	dnsStats          DNSStats
	gDNSRecords       []dnsrecords.DNSRecord
	cacheRecordsData  []dnsrecordcache.CacheRecord

	rlconfig readline.Config

	stopDNSCh    = make(chan struct{})
	stoppedDNS   = make(chan struct{})
	isServerUp   bool
	serverStatus sync.RWMutex

	appversion = "0.1.11"
)

// DNSStats holds the data for the DNS statistics
type DNSStats struct {
	TotalQueries          int `json:"total_queries"`
	TotalCacheHits        int `json:"total_cache_hits"`
	TotalBlocks           int `json:"total_blocks"`
	TotalQueriesForwarded int `json:"total_queries_forwarded"`
	TotalQueriesAnswered  int `json:"total_queries_answered"`
	ServerStartTime       time.Time
}

// DNSResolverSettings holds DNS server settings
type DNSResolverSettings struct {
	FallbackServerIP   string        `json:"fallback_server_ip"`
	FallbackServerPort string        `json:"fallback_server_port"`
	Timeout            int           `json:"timeout"`
	DNSPort            string        `json:"dns_port"`
	MDNSPort           string        `json:"mdns_port"`
	RESTPort           string        `json:"rest_port"`
	CacheRecords       bool          `json:"cache_records"`
	AutoBuildPTRFromA  bool          `json:"auto_build_ptr_from_a"`
	ForwardPTRQueries  bool          `json:"forward_ptr_queries"`
	FileLocations      fileLocations `json:"file_locations"`
}

type fileLocations struct {
	DNSServerFile  string `json:"dnsserver_file"`
	DNSRecordsFile string `json:"dnsrecords_file"`
	CacheFile      string `json:"cache_file"`
}

type cmdHelp struct {
	Name        string
	Description string
	SubCommands map[string]cmdHelp
}

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
		data.InitializeJSONFiles()

		//Load Data
		gDNSRecords = data.LoadDNSRecords()
		dnsServerSettings = loadSettings()
		cacheRecordsData = data.LoadCacheRecords()
		dnsServers = data.LoadDNSServers()

		// Set up the DNS server handler
		dns.HandleFunc(".", handleRequest)
		// Start DNS Server
		startDNSServer(*port)

		// mDNS server setup
		if *mdnsMode {
			go startMDNSServer(*mdnsPort)
		}

		// Configure readline
		rlconfig = readline.Config{
			Prompt:                 "> ",
			HistoryFile:            "/tmp/dnsresolver.history",
			DisableAutoSaveHistory: true,
			InterruptPrompt:        "^C",
			EOFPrompt:              "exit",
			HistorySearchFold:      true,
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
			rl, err := readline.NewEx(&rlconfig)
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

func startDNSServer(port string) {
	server := &dns.Server{
		Addr: fmt.Sprintf(":%s", port),
		Net:  "udp",
	}

	log.Printf("Starting DNS server on %s\n", server.Addr)
	updateServerStatus(true)
	dnsStats.ServerStartTime = time.Now()

	go func() {
		defer close(stoppedDNS)
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
		updateServerStatus(false)
	}()

	go func() {
		<-stopDNSCh
		if err := server.Shutdown(); err != nil {
			log.Fatalf("Error stopping server: %v", err)
		}
	}()
}

func restartDNSServer(port string) {
	if getServerStatus() {
		stopDNSServer()
	}
	stopDNSCh = make(chan struct{})
	stoppedDNS = make(chan struct{})

	startDNSServer(port)
}

func stopDNSServer() {
	close(stopDNSCh)
	<-stoppedDNS
	updateServerStatus(false)
}

func updateServerStatus(status bool) {
	serverStatus.Lock()
	defer serverStatus.Unlock()
	isServerUp = status
}

func getServerStatus() bool {
	serverStatus.RLock()
	defer serverStatus.RUnlock()
	return isServerUp
}
