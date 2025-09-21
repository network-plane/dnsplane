package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"dnsresolver/commandhandler"
	"dnsresolver/converters"
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsservers"
	"dnsresolver/tui"

	// "github.com/bettercap/readline"
	// "github.com/reeflective/readline"
	"github.com/chzyer/readline"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

const defaultUnixSocketPath = "/tmp/dnsresolver.socket"

var (
	rlconfig     readline.Config
	stopDNSCh    = make(chan struct{})
	stoppedDNS   = make(chan struct{})
	serverStatus sync.RWMutex
	isServerUp   bool
	appversion   = "0.1.17"
	daemonMode   bool

	rootCmd = &cobra.Command{
		Use:           "dnsresolver",
		Short:         "DNS Server with optional CLI mode",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runRoot,
	}
)

func main() {
	//Create JSON files if they don't exist
	data.InitializeJSONFiles()

	// Ensure resolver settings are initialised before running commands
	data.GetInstance().GetResolverSettings()

	rootCmd.Version = fmt.Sprintf("DNS Resolver %s", appversion)
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.String("port", "53", "Port for DNS server")
	flags.String("server-socket", defaultUnixSocketPath, "Path to UNIX domain socket for the daemon listener")
	flags.Bool("tui", false, "Run with interactive TUI")
	flags.Bool("api", false, "Enable the REST API")
	flags.String("apiport", "8080", "Port for the REST API")
	flags.StringP("client", "c", "", "Run in client mode (optional UNIX socket path)")
	if f := flags.Lookup("client"); f != nil {
		f.NoOptDefVal = defaultUnixSocketPath
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	remaining := append([]string(nil), args...)

	clientRequested := cmd.Flags().Changed("client")
	clientSocket, _ := cmd.Flags().GetString("client")
	if clientRequested {
		if clientSocket == "" {
			clientSocket = defaultUnixSocketPath
		}
		if len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
			clientSocket = remaining[0]
			remaining = remaining[1:]
		}
		if len(remaining) > 0 {
			return fmt.Errorf("unexpected arguments: %v", remaining)
		}
		connectToUnixSocket(clientSocket)
		return nil
	}

	if len(remaining) > 0 {
		return fmt.Errorf("unexpected arguments: %v", remaining)
	}

	port, _ := cmd.Flags().GetString("port")
	serverSocket, _ := cmd.Flags().GetString("server-socket")
	tuiMode, _ := cmd.Flags().GetBool("tui")
	apiMode, _ := cmd.Flags().GetBool("api")
	apiport, _ := cmd.Flags().GetString("apiport")

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.GetResolverSettings()

	daemonMode = !tuiMode

	if apiMode {
		go startGinAPI(apiport)
	}

	dns.HandleFunc(".", handleRequest)

	if port != dnsServerSettings.DNSPort {
		dnsServerSettings.DNSPort = port
	}

	if apiport != dnsServerSettings.RESTPort {
		dnsServerSettings.RESTPort = apiport
	}

	startedCh, dnsErrCh := startDNSServer(port)

	waitForServer := func() error {
		select {
		case <-startedCh:
			return nil
		case err := <-dnsErrCh:
			return err
		}
	}

	if err := waitForServer(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting DNS server: %v\n", err)
		return nil
	}

	monitorDNSErrors := func() {
		go func() {
			if err := <-dnsErrCh; err != nil {
				fmt.Fprintf(os.Stderr, "DNS server error: %v\n", err)
			}
		}()
	}

	monitorDNSErrors()

	rlconfig = readline.Config{
		Prompt:                 "> ",
		HistoryFile:            "/tmp/dnsresolver.history",
		DisableAutoSaveHistory: true,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		HistorySearchFold:      true,
	}

	if !tuiMode {
		if serverSocket != "" {
			setupUnixSocketListener(serverSocket)
		} else {
			select {}
		}
		return nil
	}

	rl, err := readline.NewEx(&rlconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return nil
	}
	rl.CaptureExitSignal()
	defer rl.Close()

	commandhandler.RegisterCommands()
	tui.SetPrompt("dnsresolver> ")
	tui.Run(rl)

	return nil
}

func startDNSServer(port string) (<-chan struct{}, <-chan error) {
	dnsData := data.GetInstance()

	server := &dns.Server{
		Addr: fmt.Sprintf(":%s", port),
		Net:  "udp",
	}

	log.Printf("Starting DNS server on %s\n", server.Addr)

	startedCh := make(chan struct{})
	errCh := make(chan error, 1)
	var once sync.Once

	server.NotifyStartedFunc = func() {
		once.Do(func() {
			updateServerStatus(true)
			stats := dnsData.GetStats()
			stats.ServerStartTime = time.Now()
			dnsData.UpdateStats(stats)
			close(startedCh)
		})
	}

	go func() {
		defer close(stoppedDNS)
		if err := server.ListenAndServe(); err != nil {
			updateServerStatus(false)
			select {
			case errCh <- err:
			default:
			}
			return
		}
		updateServerStatus(false)
	}()

	go func() {
		<-stopDNSCh
		if err := server.Shutdown(); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	return startedCh, errCh
}

func restartDNSServer(port string) {
	if getServerStatus() {
		stopDNSServer()
	}
	stopDNSCh = make(chan struct{})
	stoppedDNS = make(chan struct{})

	startedCh, errCh := startDNSServer(port)
	select {
	case <-startedCh:
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "Error restarting DNS server: %v\n", err)
	}
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

func logQuery(format string, args ...interface{}) {
	if !daemonMode {
		return
	}
	fmt.Printf(format, args...)
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
	dnsRecords := dnsdata.GetRecords()

	switch question.Qtype {
	case dns.TypePTR:
		handlePTRQuestion(question, response)
		return

	case dns.TypeA:
		recordType := dns.TypeToString[question.Qtype]
		cachedRecord := dnsrecords.FindRecord(dnsRecords, question.Name, recordType, dnsServerSettings.DNSRecordSettings.AutoBuildPTRFromA)

		if cachedRecord != nil {
			processCachedRecord(question, cachedRecord, response)
		} else {
			cachedRecord = findCacheRecord(dnsdata.GetCacheRecords(), question.Name, recordType)
			if cachedRecord != nil {
				dnsdata.IncrementCacheHits()
				processCacheRecord(question, cachedRecord, response)
			} else {

				handleDNSServers(question, dnsservers.GetDNSArray(dnsdata.DNSServers, true), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
			}
		}

	default:
		handleDNSServers(question, dnsservers.GetDNSArray(dnsdata.DNSServers, true), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
	}
	dnsdata.IncrementQueriesAnswered()
}

func handlePTRQuestion(question dns.Question, response *dns.Msg) {
	dnsdata := data.GetInstance()
	dnsServerSettings := dnsdata.GetResolverSettings()

	ipAddr := converters.ConvertReverseDNSToIP(question.Name)
	dnsRecords := dnsdata.GetRecords()
	recordType := dns.TypeToString[question.Qtype]

	rrPointer := dnsrecords.FindRecord(dnsRecords, ipAddr, recordType, dnsServerSettings.DNSRecordSettings.AutoBuildPTRFromA)
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
		logQuery("PTR record not found in dnsrecords.json\n")
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
	logQuery("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, answer.Answer[0].String(), answer.Answer[0].Header().Name[:len(answer.Answer[0].Header().Name)-1])

	cacheDNSResponse(answer)
}

func handleFallbackServer(question dns.Question, fallbackServer string, response *dns.Msg) {
	fallbackResponse, _ := queryAuthoritative(question.Name, fallbackServer)
	if fallbackResponse != nil {
		response.Answer = append(response.Answer, fallbackResponse.Answer...)
		logQuery("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, fallbackResponse.Answer[0].String(), fallbackServer)

		cacheDNSResponse(fallbackResponse)
	} else {
		logQuery("Query: %s, No response\n", question.Name)
	}
}

func cacheDNSResponse(answer *dns.Msg) {
	if answer == nil || len(answer.Answer) == 0 {
		return
	}
	cacheRRs(answer.Answer)
}

func cacheRRs(rrs []dns.RR) {
	if len(rrs) == 0 {
		return
	}

	dnsdata := data.GetInstance()
	settings := dnsdata.GetResolverSettings()
	if !settings.CacheRecords {
		return
	}

	cache := dnsdata.GetCacheRecords()
	for i := range rrs {
		rr := rrs[i]
		cache = dnsrecordcache.Add(cache, &rr)
	}
	dnsdata.UpdateCacheRecords(cache)
}

func processCachedRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	response.Authoritative = true
	logQuery("Query: %s, Reply: %s, Method: dnsrecords.json\n", question.Name, (*cachedRecord).String())
	cacheRRs([]dns.RR{*cachedRecord})
}

func processCacheRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	logQuery("Query: %s, Reply: %s, Method: dnscache.json\n", question.Name, (*cachedRecord).String())
}

func findCacheRecord(cacheRecords []dnsrecordcache.CacheRecord, name string, recordType string) *dns.RR {
	now := time.Now()
	for _, record := range cacheRecords {
		if dnsrecords.NormalizeRecordNameKey(record.DNSRecord.Name) == dnsrecords.NormalizeRecordNameKey(name) &&
			dnsrecords.NormalizeRecordType(record.DNSRecord.Type) == dnsrecords.NormalizeRecordType(recordType) {
			if now.Before(record.Expiry) {
				remainingTTL := uint32(record.Expiry.Sub(now).Seconds())
				return dnsRecordToRR(&record.DNSRecord, remainingTTL)
			}
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

	logQuery("response %s\n", response.Answer[0].String())

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
				if errors.Is(err, io.EOF) {
					return
				}
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
	rl.CaptureExitSignal()
	defer rl.Close() // Close readline when done

	commandhandler.RegisterCommands()
	tui.SetPrompt("dnsresolver> ")
	tui.Run(rl)
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

	dnsrecords.Add(request, dnsData.DNSRecords, false) // Call the existing addRecord function with parsed input
	c.JSON(201, gin.H{"status": "Record added"})
}

// Wrapper for existing listRecords function
func listRecordsGin(c *gin.Context) {
	dnsData := data.GetInstance()
	// Call the existing listRecords function with no args
	dnsrecords.List(dnsData.DNSRecords, []string{})
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
