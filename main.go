// Description: Main entry point for the DNS Resolver application.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dnsresolver/converters"
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsservers"

	// "github.com/bettercap/readline"
	"github.com/chzyer/readline"
	"github.com/gin-gonic/gin"
	cli "github.com/jawher/mow.cli"
	"github.com/miekg/dns"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	rlconfig readline.Config

	stopDNSCh    = make(chan struct{})
	stoppedDNS   = make(chan struct{})
	isServerUp   bool
	serverStatus sync.RWMutex

	appversion = "0.1.13"
)

func main() {
	//Create JSON files if they don't exist
	data.InitializeJSONFiles()

	// Initialize data
	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.GetResolverSettings()

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

		// Set up the DNS server handler
		dns.HandleFunc(".", handleRequest)

		//handle settings overriden by command line
		if *port != dnsServerSettings.DNSPort {
			dnsServerSettings.DNSPort = *port
		}

		if *mdnsPort != dnsServerSettings.MDNSPort {
			dnsServerSettings.MDNSPort = *mdnsPort
		}

		if *apiport != dnsServerSettings.RESTPort {
			dnsServerSettings.RESTPort = *apiport
		}

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
	dnsData := data.GetInstance()

	server := &dns.Server{
		Addr: fmt.Sprintf(":%s", port),
		Net:  "udp",
	}

	log.Printf("Starting DNS server on %s\n", server.Addr)
	updateServerStatus(true)

	// Update the server start time
	stats := dnsData.GetStats()
	stats.ServerStartTime = time.Now()
	dnsData.UpdateStats(stats)

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

//stats

// serverUpTimeFormat formats the time duration since the server start time into a human-readable string.
func serverUpTimeFormat(startTime time.Time) string {
	duration := time.Since(startTime)

	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes, %d seconds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes, %d seconds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func showStats() {
	dnsData := data.GetInstance()

	fmt.Println("Stats:")
	fmt.Println("Server start time:", dnsData.Stats.ServerStartTime)
	fmt.Println("Server Up Time:", serverUpTimeFormat(dnsData.Stats.ServerStartTime))
	fmt.Println()
	fmt.Println("Total Records:", len(dnsData.DNSRecords))
	fmt.Println("Total DNS Servers:", len(data.LoadDNSServers()))
	fmt.Println("Total Cache Records:", len(dnsData.CacheRecords))
	fmt.Println()
	fmt.Println("Total queries received:", dnsData.Stats.TotalQueries)
	fmt.Println("Total queries answered:", dnsData.Stats.TotalQueriesAnswered)
	fmt.Println("Total cache hits:", dnsData.Stats.TotalCacheHits)
	fmt.Println("Total queries forwarded:", dnsData.Stats.TotalQueriesForwarded)
}

// mdns server
func startMDNSServer(port string) {
	portInt, _ := strconv.Atoi(port)

	// Set up the multicast address for mDNS
	addr := &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: portInt}

	// Create a UDP connection to listen on multicast address
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Fatalf("Error setting up mDNS server: %v", err)
	}

	// Set reuse address option to allow multiple listeners on the same address
	if err := conn.SetReadBuffer(65535); err != nil {
		log.Fatalf("Failed to set read buffer size: %v", err)
	}

	server := &dns.Server{
		PacketConn: conn,
	}

	dns.HandleFunc("local.", handleMDNSRequest)

	log.Printf("Starting mDNS server on %s\n", addr)
	if err := server.ActivateAndServe(); err != nil {
		log.Fatalf("Error starting mDNS server: %v", err)
	}
}

// This is JUST a test, it will always return the same IP :P
func handleMDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	log.Printf("Received mDNS query: %s %s\n", q.Name, dns.TypeToString[q.Qtype])

	// Check if the request is for the .local domain
	if q.Qclass != dns.ClassINET || !dns.IsSubDomain("local.", q.Name) {
		log.Printf("Not an mDNS query, ignoring: %s\n", q.Name)
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch q.Qtype {
	case dns.TypeA:
		// Return IPv4 address for A query
		ipv4 := net.ParseIP("127.0.0.1")
		if ipv4 == nil {
			log.Printf("Invalid IPv4 address provided\n")
			return
		}
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
			A:   ipv4,
		})
	case dns.TypeAAAA:
		// Return IPv6 address for AAAA query
		ipv6 := net.ParseIP("::1")
		if ipv6 == nil {
			log.Printf("Invalid IPv6 address provided\n")
			return
		}
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 120},
			AAAA: ipv6,
		})
	default:
		log.Printf("Unsupported mDNS query type: %d\n", q.Qtype)
		return
	}

	// Write the response to the multicast address
	if err := w.WriteMsg(m); err != nil {
		log.Printf("Failed to write mDNS response: %v\n", err)
	}
}

func isMDNSQuery(name string) bool {
	// Check if the query is for the .local domain
	if strings.HasSuffix(name, ".local.") {
		return true
	}

	// Split the query name by dots
	parts := strings.Split(name, ".")

	// Check if the query has at least four parts (minimum for an mDNS query)
	if len(parts) < 3 {
		return false
	}

	return false
}

// DNS
func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = false

	dnsData := data.GetInstance()
	dnsData.IncrementTotalQueries()

	for _, question := range request.Question {
		handleQuestion(question, response)
	}

	err := writer.WriteMsg(response)
	if err != nil {
		log.Println("Error writing response:", err)
	}
}

func handleQuestion(question dns.Question, response *dns.Msg) {
	dnsdata := data.GetInstance()
	dnsServerSettings := dnsdata.GetResolverSettings()

	dnsData := data.GetInstance()

	switch question.Qtype {
	case dns.TypePTR:
		handlePTRQuestion(question, response)
		return

	case dns.TypeA:
		recordType := dns.TypeToString[question.Qtype]
		cachedRecord := findRecord(dnsdata.DNSRecords, question.Name, recordType)

		if cachedRecord != nil {
			processCachedRecord(question, cachedRecord, response)
		} else {
			cachedRecord = findCacheRecord(dnsData.CacheRecords, question.Name, recordType)
			if cachedRecord != nil {
				dnsData.IncrementCacheHits()
				processCacheRecord(question, cachedRecord, response)
			} else {

				handleDNSServers(question, dnsservers.GetDNSArray(dnsdata.DNSServers, true), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
			}
		}

	default:
		handleDNSServers(question, dnsservers.GetDNSArray(dnsdata.DNSServers, true), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
	}
	dnsData.IncrementQueriesAnswered()
}

func handlePTRQuestion(question dns.Question, response *dns.Msg) {
	dnsdata := data.GetInstance()
	dnsServerSettings := dnsdata.GetResolverSettings()

	ipAddr := converters.ConvertReverseDNSToIP(question.Name)
	dnsRecords := data.LoadDNSRecords()
	recordType := dns.TypeToString[question.Qtype]

	rrPointer := findRecord(dnsRecords, ipAddr, recordType)
	if rrPointer != nil {
		ptrRecord, ok := (*rrPointer).(*dns.PTR)
		if !ok {
			// Handle the case where the record is not a PTR record or cannot be cast
			log.Println("Found record is not a PTR record or cannot be cast to *dns.PTR")
			return
		}

		ptrDomain := ptrRecord.Ptr
		if !strings.HasSuffix(ptrDomain, ".") {
			ptrDomain += "."
		}

		// Now use ptrDomain in the sprintf, ensuring only one trailing dot is present
		rrString := fmt.Sprintf("%s PTR %s", question.Name, ptrDomain)
		rr, err := dns.NewRR(rrString)
		if err == nil {
			response.Answer = append(response.Answer, rr)
		} else {
			// Log the error
			log.Printf("Error creating PTR record: %s\n", err)
		}

	} else {
		fmt.Println("PTR record not found in dnsrecords.json")
		handleDNSServers(question, dnsservers.GetDNSArray(dnsdata.DNSServers, true), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
	}
}

func dnsRecordToRR(dnsRecord *dnsrecords.DNSRecord, ttl uint32) *dns.RR {
	recordString := fmt.Sprintf("%s %d IN %s %s", dnsRecord.Name, ttl, dnsRecord.Type, dnsRecord.Value)
	rr, err := dns.NewRR(recordString)
	if err != nil {
		log.Printf("Error converting DnsRecord to dns.RR: %s\n", err)
		return nil
	}
	return &rr
}

func processAuthoritativeAnswer(question dns.Question, answer *dns.Msg, response *dns.Msg) {
	dnsdata := data.GetInstance()

	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, answer.Answer[0].String(), answer.Answer[0].Header().Name[:len(answer.Answer[0].Header().Name)-1])

	data.SaveCacheRecords(dnsdata.CacheRecords)
}

func handleFallbackServer(question dns.Question, fallbackServer string, response *dns.Msg) {
	dnsdata := data.GetInstance()

	fallbackResponse, _ := queryAuthoritative(question.Name, fallbackServer)
	if fallbackResponse != nil {
		response.Answer = append(response.Answer, fallbackResponse.Answer...)
		fmt.Printf("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, fallbackResponse.Answer[0].String(), fallbackServer)

		data.SaveCacheRecords(dnsdata.CacheRecords)
	} else {
		fmt.Printf("Query: %s, No response\n", question.Name)
	}
}

func processCachedRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: dnsrecords.json\n", question.Name, (*cachedRecord).String())
}

func processCacheRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	fmt.Printf("Query: %s, Reply: %s, Method: dnscache.json\n", question.Name, (*cachedRecord).String())
}

func findCacheRecord(cacheRecords []dnsrecordcache.CacheRecord, name string, recordType string) *dns.RR {
	now := time.Now()
	for _, record := range cacheRecords {
		if record.DNSRecord.Name == name && record.DNSRecord.Type == recordType {
			if now.Before(record.Expiry) {
				remainingTTL := uint32(record.Expiry.Sub(now).Seconds())
				return dnsRecordToRR(&record.DNSRecord, remainingTTL)
			}
		}
	}
	return nil
}

func findRecord(records []dnsrecords.DNSRecord, lookupRecord, recordType string) *dns.RR {
	dnsdata := data.GetInstance()
	dnsServerSettings := dnsdata.GetResolverSettings()

	for _, record := range records {

		if record.Type == "PTR" || (recordType == "PTR" && dnsServerSettings.AutoBuildPTRFromA) {
			if record.Value == lookupRecord {
				recordString := fmt.Sprintf("%s %d IN PTR %s.", converters.ConvertIPToReverseDNS(lookupRecord), record.TTL, strings.TrimRight(record.Name, "."))
				fmt.Println("recordstring", recordString)

				rr := recordString
				dnsRecord, err := dns.NewRR(rr)
				if err != nil {
					fmt.Println("Error creating PTR record", err)
					return nil // Error handling if the PTR record can't be created
				}
				// fmt.Println(dnsRecord.String())
				return &dnsRecord
			}
		}

		if record.Name == lookupRecord && record.Type == recordType {
			rr := fmt.Sprintf("%s %d IN %s %s", record.Name, record.TTL, record.Type, record.Value)
			dnsRecord, err := dns.NewRR(rr)
			if err != nil {
				return nil
			}
			return &dnsRecord
		}
	}
	return nil
}

func queryAuthoritative(questionName string, server string) (*dns.Msg, error) {
	client := new(dns.Client)
	client.Timeout = 2 * time.Second // Set the desired timeout duration
	message := new(dns.Msg)
	message.SetQuestion(questionName, dns.TypeA)
	response, _, err := client.Exchange(message, server)
	if err != nil {
		log.Printf("Error querying DNS server (%s) for %s: %s\n", server, questionName, err)
		return nil, err
	}

	if len(response.Answer) == 0 {
		log.Printf("No answer received from DNS server (%s) for %s\n", server, questionName)
		return nil, errors.New("no answer received")
	}

	fmt.Println("response", response.Answer[0].String())

	return response, nil
}

func queryAllDNSServers(question dns.Question, dnsServers []string) <-chan *dns.Msg {
	answers := make(chan *dns.Msg, len(dnsServers))
	var wg sync.WaitGroup

	for _, server := range dnsServers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			authResponse, _ := queryAuthoritative(question.Name, server)
			if authResponse != nil {
				answers <- authResponse
			}
		}(server)
	}

	go func() {
		wg.Wait()
		close(answers)
	}()

	return answers
}

func handleDNSServers(question dns.Question, dnsServers []string, fallbackServer string, response *dns.Msg) {
	answers := queryAllDNSServers(question, dnsServers)

	found := false
	for answer := range answers {
		if answer.MsgHdr.Authoritative {
			processAuthoritativeAnswer(question, answer, response)
			found = true
			break
		}
	}

	if !found {
		handleFallbackServer(question, fallbackServer, response)
	}
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
		}(conn) // Start the goroutine with the current connection
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

	// Interactive mode setup
	rl, err := readline.NewEx(&rlconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return
	}
	defer rl.Close() // Close readline when done

	handleCommandLoop(rl) // Call the function for command handling
}

// api
// Wrapper for existing addRecord function
func addRecordGin(c *gin.Context) {
	dnsData := data.GetInstance()
	// Read command from JSON request body
	var request []string
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	dnsrecords.Add(request, dnsData.DNSRecords) // Call the existing addRecord function with parsed input
	c.JSON(201, gin.H{"status": "Record added"})
}

// Wrapper for existing listRecords function
func listRecordsGin(c *gin.Context) {
	dnsData := data.GetInstance()
	// Call the existing listRecords function
	dnsrecords.List(dnsData.DNSRecords)
	c.JSON(200, gin.H{"status": "Listed"})
}

func startGinAPI(apiport string) {
	// Create a Gin router
	r := gin.Default()

	// Add routes for the API
	r.GET("/dns/records", listRecordsGin) // List all DNS records
	r.POST("/dns/records", addRecordGin)  // Add a new DNS record

	// Start the server
	if err := r.Run(fmt.Sprintf(":%s", apiport)); err != nil {
		log.Fatal("Error starting API:", err)
	}
}

// Command handling
type cmdHelp struct {
	Name        string
	Description string
	SubCommands map[string]cmdHelp
}

// Map to exclude commands from saving to history
var excludedCommands = map[string]bool{
	"q": true, "quit": true, "exit": true, "h": true, "help": true, "ls": true, "l": true, "/": true,
}

// Handle the command loop for reading and processing user input
func handleCommandLoop(rl *readline.Instance) {
	var currentContext string
	setupAutocomplete(rl, currentContext)

	for {
		updatePrompt(rl, currentContext)
		command, err := rl.Readline()
		if err != nil {
			break
		}

		command = strings.TrimSpace(command)
		args := strings.Fields(command)
		if len(args) == 0 {
			continue
		}

		if isExitCommand(args[0]) {
			fmt.Println("Shutting down.")
			os.Exit(0)
		}

		if !excludedCommands[args[0]] {
			if err := rl.SaveHistory(command); err != nil {
				fmt.Println("Error saving history:", err)
			}
		}

		if currentContext == "" {
			handleGlobalCommands(args, rl, &currentContext)
		} else {
			handleSubcommands(args, rl, &currentContext)
		}
	}
}

// Check if the command is an exit command
func isExitCommand(cmd string) bool {
	return cmd == "q" || cmd == "quit" || cmd == "exit"
}

// Handle global commands
func handleGlobalCommands(args []string, rl *readline.Instance, currentContext *string) {
	switch args[0] {
	case "stats":
		handleStats()
	case "record", "cache", "dns", "server":
		handleContextCommand(args[0], args, rl, currentContext)
	case "help", "h", "?", "ls", "l":
		showHelp("")
	case "exit", "quit", "q":
		fmt.Println("Shutting down.")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", args[0])
	}
}

// Handle server-specific commands
func handleServerCommand(args []string, rl *readline.Instance, currentContext *string) {
	if len(args) > 1 {
		switch args[1] {
		case "start", "stop", "status", "configure":
			handleContextCommand(args[1], args[1:], rl, currentContext)
			return
		}
	}
	handleContextCommand(args[0], args, rl, currentContext)
}

// Handle context-based commands
func handleContextCommand(command string, args []string, rl *readline.Instance, currentContext *string) {
	if len(args) > 1 {
		handleSubcommand(command, args, *currentContext)
	} else {
		*currentContext = command
		setupAutocomplete(rl, *currentContext)
	}
}

// Handle subcommands based on the current context
func handleSubcommands(args []string, rl *readline.Instance, currentContext *string) {
	if args[0] == "/" {
		*currentContext = "" // Change context back to global
		setupAutocomplete(rl, *currentContext)
		return
	}

	if args[0] == "help" || args[0] == "?" {
		showHelp(*currentContext)
		return
	}

	handleSubcommand(*currentContext, args, *currentContext)
}

// Dispatch subcommands to the appropriate handlers
func handleSubcommand(command string, args []string, context string) {
	handlers := map[string]func([]string){
		"record": handleRecord,
		"cache":  handleCache,
		"dns":    handleDNS,
		"server": handleServer,
	}

	if handler, ok := handlers[command]; ok {
		handler(args[1:]) // Pass all arguments except the first (command name)
	} else {
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
	}
}

// Handlers for the commands
func handleStats() {
	showStats()
}

func handleCommand(args []string, context string, commands map[string]func([]string)) {
	if len(args) == 0 {
		fmt.Printf("%s subcommand required. Use '%s ?' for help.\n", context, context)
		return
	}

	subCmd := args[0]
	if !checkHelp(subCmd, context) {
		// checkHelp returns false if it's a help command and has already displayed help
		return
	}

	if handler, ok := commands[subCmd]; ok {
		// Pass the entire args slice to the handler
		// The handler should handle cases where no additional arguments are provided
		handler(args)
	} else {
		fmt.Printf("Unknown %s subcommand: %s. Use '%s ?' for help.\n", context, subCmd, context)
	}
}

func handleRecord(args []string) {
	dnsData := data.GetInstance()
	gDNSRecords := dnsData.DNSRecords
	commands := map[string]func([]string){
		"add":    func(args []string) { gDNSRecords = dnsrecords.Add(args, gDNSRecords) },
		"remove": func(args []string) { gDNSRecords = dnsrecords.Remove(args, gDNSRecords) },
		"update": func(args []string) { dnsrecords.Update(args, gDNSRecords) },
		"list":   func(args []string) { dnsrecords.List(gDNSRecords) },
		"clear":  func(args []string) { gDNSRecords = []dnsrecords.DNSRecord{} },
		"load":   func(args []string) { data.LoadDNSRecords() },
		"save":   func(args []string) { data.SaveDNSRecords(gDNSRecords) },
	}
	dnsData.UpdateRecords(gDNSRecords)
	handleCommand(args, "record", commands)
}

func handleCache(args []string) {
	dnsdata := data.GetInstance()
	cacheRecordsData := dnsdata.CacheRecords

	commands := map[string]func([]string){
		"list":   func(args []string) { dnsrecordcache.List(cacheRecordsData) },
		"remove": func(args []string) { cacheRecordsData = dnsrecordcache.Remove(args, cacheRecordsData) },
		"clear":  func(args []string) { cacheRecordsData = []dnsrecordcache.CacheRecord{} },
		"load":   func(args []string) { data.LoadCacheRecords() },
		"save":   func(args []string) { data.SaveCacheRecords(cacheRecordsData) },
	}
	dnsdata.UpdateCacheRecords(cacheRecordsData)
	handleCommand(args, "cache", commands)
}

func handleDNS(args []string) {
	dnsData := data.GetInstance()
	dnsServers := dnsData.DNSServers
	commands := map[string]func([]string){
		"add":    func(args []string) { dnsServers = dnsservers.Add(args, dnsServers) },
		"remove": func(args []string) { dnsServers = dnsservers.Remove(args, dnsServers) },
		"update": func(args []string) { dnsServers = dnsservers.Update(args, dnsServers) },
		"list":   func(args []string) { dnsservers.List(dnsServers) },
		"clear":  func(args []string) { dnsServers = []dnsservers.DNSServer{} },
		"load":   func(args []string) { data.LoadDNSServers() },
		"save":   func(args []string) { data.SaveDNSServers(dnsServers) },
	}
	dnsData.UpdateServers(dnsServers)
	handleCommand(args, "dns", commands)
}

func handleServer(args []string) {
	if len(args) == 0 {
		fmt.Println("Server subcommand required. Use 'server ?' for help.")
		return
	}

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	serverCommands := map[string]func([]string){
		"start":     handleServerStart,
		"stop":      handleServerStop,
		"status":    handleServerStatus,
		"configure": handleServerConfigure,
		"load": func(args []string) {
			dnsServerSettings = data.LoadSettings()
			dnsData.UpdateSettings(dnsServerSettings)
		},
		"save": func(args []string) { data.SaveSettings(dnsServerSettings) },
	}

	if cmd, ok := serverCommands[args[0]]; ok {
		cmd(args[1:])
	} else {
		fmt.Printf("Unknown server subcommand: %s. Use 'server ?' for help.\n", args[0])
	}
}

func handleServerStart(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to start required. Use 'server start ?' for help.")
		return
	}

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	startCommands := map[string]func(){
		"dns":  func() { restartDNSServer(dnsServerSettings.DNSPort) },
		"mdns": func() { startMDNSServer(dnsServerSettings.MDNSPort) },
		"api":  func() { startGinAPI(dnsServerSettings.RESTPort) },
		"dhcp": func() { /* startDHCP() */ },
	}

	if cmd, ok := startCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to start: %s. Use 'server start ?' for help.\n", args[0])
	}
}

func handleServerStop(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to stop required. Use 'server stop ?' for help.")
		return
	}

	stopCommands := map[string]func(){
		"dns":  func() { stopDNSServer() },
		"mdns": func() { /* stopMDNSServer() */ },
		"api":  func() { /* stopGinAPI() */ },
		"dhcp": func() { /* stopDHCP() */ },
	}

	if cmd, ok := stopCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to stop: %s. Use 'server stop ?' for help.\n", args[0])
	}
}

func handleServerStatus(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component required. Use 'server status ?' for help.")
		return
	}

	statusCommands := map[string]func(){
		"dns":  func() { fmt.Println("DNS Server Status: ", getServerStatus()) },
		"mdns": func() { /* getMDNSStatus() */ },
		"api":  func() { /* getAPIStatus() */ },
		"dhcp": func() { /* getDHCPStatus() */ },
	}

	if cmd, ok := statusCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown status component: %s. Use 'server status ?' for help.\n", args[0])
	}
}

func handleServerConfigure(args []string) {
	// Placeholder for server configuration
	fmt.Println("Server configuration not implemented yet")
}

func setupAutocomplete(rl *readline.Instance, context string) {
	updatePrompt(rl, context)

	completer := readline.NewPrefixCompleter(
		readline.PcItem("record", readline.PcItemDynamic(func(_ string) []string { return getRecordItems() })),
		readline.PcItem("cache", readline.PcItemDynamic(func(_ string) []string { return getCacheItems() })),
		readline.PcItem("dns", readline.PcItemDynamic(func(_ string) []string { return getDNSItems() })),
		readline.PcItem("server", readline.PcItemDynamic(func(_ string) []string { return getServerItems() })),
		readline.PcItem("stats"),
		readline.PcItem("help"),
		readline.PcItem("h"),
		readline.PcItem("?"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("q"),
	)

	rl.Config.AutoComplete = completer
}

func getRecordItems() []string {
	return []string{"add", "remove", "update", "list", "clear", "load", "save", "?"}
}

func getCacheItems() []string {
	return []string{"list", "remove", "clear", "load", "save", "?"}
}

func getDNSItems() []string {
	return []string{"add", "remove", "update", "list", "clear", "load", "save", "?"}
}

func getServerItems() []string {
	return []string{"start", "stop", "status", "configure", "load", "save", "?"}
}

func updatePrompt(rl *readline.Instance, currentContext string) {
	prompt := "> "
	if currentContext != "" {
		prompt = fmt.Sprintf("(%s) > ", currentContext)
	}
	rl.SetPrompt(prompt)
	rl.Refresh()
}

//help

func showHelp(context string) {
	commands := loadCommands()
	caser := cases.Title(language.English)

	if context == "" {
		fmt.Println("Available commands:")
		helpPrinter(commands, false, false)
		commonHelp(false)
	} else if cmd, exists := commands[context]; exists {
		fmt.Printf("%s commands:\n", caser.String(context))
		helpPrinter(map[string]cmdHelp{context: cmd}, true, true)
		commonHelp(true)
	} else {
		fmt.Printf("Unknown context: %s. Available commands:\n", context)
		helpPrinter(commands, false, false)
		commonHelp(false)
	}
}

// helpPrinter prints help for commands, optionally including subcommands.
func helpPrinter(commands map[string]cmdHelp, includeSubCommands bool, isSubCmd bool) {
	var lines []string

	for _, cmd := range commands {
		if !isSubCmd {
			lines = append(lines, fmt.Sprintf("%-15s %s", cmd.Name, cmd.Description))
		}

		if includeSubCommands && len(cmd.SubCommands) > 0 {
			for _, subCmd := range cmd.SubCommands {
				lines = append(lines, fmt.Sprintf("  %-15s %s", subCmd.Name, subCmd.Description))
			}
		}
	}

	sort.Strings(lines)

	for _, line := range lines {
		fmt.Println(line)
	}
}

// commonHelp prints common help commands.
func commonHelp(indent bool) {
	indentation := ""
	if indent {
		indentation = "  "
	}
	fmt.Printf("%s%-15s %s\n", indentation, "/", "- Go up one level")
	fmt.Printf("%s%-15s %s\n", indentation, "exit, quit, q", "- Shutdown the server")
	fmt.Printf("%s%-15s %s\n", indentation, "help, h, ?", "- Show help")
}

// mainHelp displays the available commands without subcommands.
func mainHelp() {
	fmt.Println("Available commands:")
	helpPrinter(loadCommands(), false, false)
	commonHelp(false) // Add common help commands
}

// checkHelp determines if the argument is for help.
func checkHelp(arg, currentSub string) bool {
	checkArgs := []string{"?", "help", "h", "ls", "l"}

	for _, cmd := range checkArgs {
		if arg == cmd {
			subCommandHelp(currentSub)
			return false
		}
	}

	return true
}

// subCommandHelp prints help for a specific context or for all contexts if none is provided.
func subCommandHelp(context string) {
	commands := loadCommands()

	if context == "" {
		// Print only top-level commands if context is empty
		helpPrinter(commands, false, false)
		commonHelp(false)
	} else if cmd, exists := commands[context]; exists {
		// Print only subcommands if the context is found
		helpPrinter(map[string]cmdHelp{context: cmd}, true, true)
		commonHelp(true)
	} else {
		fmt.Println("Unknown context:", context)
		commonHelp(false)
	}
}

// loadCommands returns a map with command information.
func loadCommands() map[string]cmdHelp {
	return map[string]cmdHelp{
		"record": {
			Name:        "record",
			Description: "- Record Management",
			SubCommands: map[string]cmdHelp{
				"add":    {"add", "- Add a new DNS record", nil},
				"remove": {"remove", "- Remove a DNS record", nil},
				"update": {"update", "- Update a DNS record", nil},
				"list":   {"list", "- List all DNS records", nil},
				"clear":  {"clear", "- Clear all DNS records", nil},
				"load":   {"load", "- Load DNS records from a file", nil},
				"save":   {"save", "- Save DNS records to a file", nil},
			},
		},
		"cache": {
			Name:        "cache",
			Description: "- Cache Management",
			SubCommands: map[string]cmdHelp{
				"remove": {"remove", "- Remove an entry", nil},
				"list":   {"list", "- List all cache entries", nil},
				"clear":  {"clear", "- Clear the cache", nil},
				"load":   {"load", "- Load Cache records from a file", nil},
				"save":   {"save", "- Save Cache records to a file", nil},
			},
		},
		"dns": {
			Name:        "dns",
			Description: "- DNS Server Management",
			SubCommands: map[string]cmdHelp{
				"add":    {"add", "- Add a new DNS server", nil},
				"remove": {"remove", "- Remove a DNS server", nil},
				"update": {"update", "- Update a DNS server", nil},
				"list":   {"list", "- List all DNS servers", nil},
				"clear":  {"clear", "- Clear all DNS servers", nil},
				"load":   {"load", "- Load DNS servers from a file", nil},
				"save":   {"save", "- Save DNS servers to a file", nil},
			},
		},
		"server": {
			Name:        "server",
			Description: "- Server Management",
			SubCommands: map[string]cmdHelp{
				"start":     {"start", "- Start the server", nil},
				"stop":      {"stop", "- Stop the server", nil},
				"status":    {"status", "- Show server status", nil},
				"configure": {"configure", "- Set/List configuration", nil},
				"save":      {"save", "- Save the current settings", nil},
				"load":      {"load", "- Load the settings from the files", nil},
			},
		},
	}
}
