// Description: Main entry point for the DNS Resolver application.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
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
)

var (
	// gDNSRecords      []dnsrecords.DNSRecord
	cacheRecordsData []dnsrecordcache.CacheRecord

	rlconfig readline.Config

	stopDNSCh    = make(chan struct{})
	stoppedDNS   = make(chan struct{})
	isServerUp   bool
	serverStatus sync.RWMutex

	appversion = "0.1.11"
)

func main() {
	//Create JSON files if they don't exist
	data.InitializeJSONFiles()

	// Initialize data
	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.GetResolverSettings()
	//Load Data
	// gDNSRecords = data.LoadDNSRecords()

	cacheRecordsData = data.LoadCacheRecords()

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
	fmt.Println("Total Cache Records:", len(cacheRecordsData))
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
			cachedRecord = findCacheRecord(cacheRecordsData, question.Name, recordType)
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
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, answer.Answer[0].String(), answer.Answer[0].Header().Name[:len(answer.Answer[0].Header().Name)-1])

	data.SaveCacheRecords(cacheRecordsData)
}

func handleFallbackServer(question dns.Question, fallbackServer string, response *dns.Msg) {
	fallbackResponse, _ := queryAuthoritative(question.Name, fallbackServer)
	if fallbackResponse != nil {
		response.Answer = append(response.Answer, fallbackResponse.Answer...)
		fmt.Printf("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, fallbackResponse.Answer[0].String(), fallbackServer)

		data.SaveCacheRecords(cacheRecordsData)
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
