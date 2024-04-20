package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bettercap/readline"
	"github.com/miekg/dns"
)

func main() {
	initializeJSONFiles()

	dns.HandleFunc(".", handleRequest)

	dnsServerSettings = getSettings()

	server := &dns.Server{
		Addr: ":53",
		Net:  "udp",
	}

	go func() {
		log.Printf("Starting DNS server on %s\n", server.Addr)
		dnsStats.ServerStartTime = time.Now()
		err := server.ListenAndServe()
		if err != nil {
			fmt.Println("Error starting server:", err)
			os.Exit(1)
		}
	}()

	config := readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/readline_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	}

	rl, err := readline.NewEx(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return
	}
	defer rl.Close()

	// Command auto-completion setup
	rl.Config.AutoComplete = readline.NewPrefixCompleter(
		readline.PcItem("stats"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("q"),
		readline.PcItem("help"),
		readline.PcItem("h"),
		readline.PcItem("?"),
	)

	for {
		command, err := rl.Readline()
		if err != nil { // Handle EOF or interrupt
			break
		}
		fullCommand := strings.Fields(command)
		command = strings.TrimSpace(command)

		if len(fullCommand) > 0 {
			// Retrieve only the first word (command)
			command = fullCommand[0]
		} else {
			// No input or only spaces were entered
			continue
		}

		switch command {
		case "stats":
			showStats()
		case "add":
			//add DNS record
			if len(fullCommand) > 1 && fullCommand[1] == "?" {
				fmt.Println("Enter the DNS record in the format: Name Type Value TTL")
				fmt.Println("Example: example.com A 127.0.0.1 3600")
				continue
			}

			if len(fullCommand) != 5 {
				fmt.Println("Invalid DNS record format. Please enter the DNS record in the format: Name Type Value TTL")
				continue
			}

			//check if type (fullCommand[2]) is valid for DNS type
			if _, ok := dns.StringToType[fullCommand[2]]; !ok {
				fmt.Println("Invalid DNS record type. Please enter a valid DNS record type.")
				continue
			}

			ttl64, err := strconv.ParseUint(fullCommand[4], 10, 32)
			if err != nil {
				fmt.Println("Invalid TTL value. Please enter a valid TTL value.")
				continue
			}
			ttl := uint32(ttl64)

			dnsRecord := DNSRecord{
				Name:  fullCommand[1],
				Type:  fullCommand[2],
				Value: fullCommand[3],
				TTL:   ttl,
			}

			fmt.Println("Adding DNS record:", dnsRecord)

		case "exit", "quit", "q":
			fmt.Println("Shutting down.")
			return
		case "":
			continue
		case "help", "h", "?":
			fmt.Println("Available commands:")
			fmt.Println("  stats - Show server statistics")
			fmt.Println("  exit, quit, q - Shutdown the server")
			fmt.Println("  help, h, ? - Show help")
		default:
			if command != "" {
				fmt.Println("Unknown command:", command)
			}
		}
	}
}

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
	// Implement this function based on your needs
	fmt.Println("Stats:")
	fmt.Println("Server start time:", dnsStats.ServerStartTime)
	fmt.Println("Server Up Time:", serverUpTimeFormat(dnsStats.ServerStartTime))
	fmt.Println()
	fmt.Println("Total A Records:", len(getDNSRecords()))
	fmt.Println("Total DNS Servers:", len(getDNSServers()))
	// fmt.Println("Total Cache Records:", len(getCacheRecords()))
	fmt.Println()
	fmt.Println("Total queries received:", dnsStats.TotalQueries)
	fmt.Println("Total queries answered:", dnsStats.TotalQueriesAnswered)
	fmt.Println("Total cache hits:", dnsStats.TotalCacheHits)
	fmt.Println("Total queries forwarded:", dnsStats.TotalQueriesForwarded)
}

func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = false
	dnsStats.TotalQueries++

	for _, question := range request.Question {
		handleQuestion(question, response)
	}

	err := writer.WriteMsg(response)
	if err != nil {
		log.Println("Error writing response:", err)
	}
}

func handleQuestion(question dns.Question, response *dns.Msg) {
	dnsRecords := getDNSRecords()
	dnsServers := getDNSServers()
	cacheRecords, err := getCacheRecords()
	if err != nil {
		log.Println("Error getting cache records:", err)
	}

	switch question.Qtype {
	case dns.TypePTR:
		handlePTRQuestion(question, response)
		return

	case dns.TypeA:
		recordType := dns.TypeToString[question.Qtype]
		cachedRecord := findRecord(dnsRecords, question.Name, recordType)

		if cachedRecord != nil {
			processCachedRecord(question, cachedRecord, response)
		} else {
			cachedRecord = findCacheRecord(cacheRecords, question.Name, recordType)
			if cachedRecord != nil {
				dnsStats.TotalCacheHits++
				processCacheRecord(question, cachedRecord, response)
			} else {
				handleDNSServers(question, dnsServers, fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
			}
		}

	default:
		handleDNSServers(question, dnsServers, fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
	}
	dnsStats.TotalQueriesAnswered++
}

func handleAQuestion() {

}

func handlePTRQuestion(question dns.Question, response *dns.Msg) {
	ipAddr := convertReverseDNSToIP(question.Name)
	dnsRecords := getDNSRecords()
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
		fmt.Println("PTR record not found in records.json")
		handleDNSServers(question, getDNSServers(), fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
	}
}

func processCachedRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: records.json\n", question.Name, (*cachedRecord).String())
}

func processCacheRecord(question dns.Question, cachedRecord *dns.RR, response *dns.Msg) {
	response.Answer = append(response.Answer, *cachedRecord)
	fmt.Printf("Query: %s, Reply: %s, Method: cache.json\n", question.Name, (*cachedRecord).String())
}

func processAuthoritativeAnswer(question dns.Question, answer *dns.Msg, response *dns.Msg) {
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, answer.Answer[0].String(), answer.Answer[0].Header().Name[:len(answer.Answer[0].Header().Name)-1])

	// Cache the authoritative answers
	cacheRecords, err := getCacheRecords()
	if err != nil {
		log.Println("Error getting cache records:", err)
	}
	for _, authoritativeAnswer := range answer.Answer {
		cacheRecords = addToCache(cacheRecords, &authoritativeAnswer)
	}
	saveCacheRecords(cacheRecords)
}

func handleFallbackServer(question dns.Question, fallbackServer string, response *dns.Msg) {
	fallbackResponse, _ := queryAuthoritative(question.Name, fallbackServer)
	if fallbackResponse != nil {
		response.Answer = append(response.Answer, fallbackResponse.Answer...)
		fmt.Printf("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, fallbackResponse.Answer[0].String(), fallbackServer)

		// Cache the fallback server answers
		cacheRecords, err := getCacheRecords()
		if err != nil {
			log.Println("Error getting cache records:", err)
		}
		for _, fallbackAnswer := range fallbackResponse.Answer {
			cacheRecords = addToCache(cacheRecords, &fallbackAnswer)
		}
		saveCacheRecords(cacheRecords)
	} else {
		fmt.Printf("Query: %s, No response\n", question.Name)
	}
}

func dnsRecordToRR(dnsRecord *DNSRecord, ttl uint32) *dns.RR {
	recordString := fmt.Sprintf("%s %d IN %s %s", dnsRecord.Name, ttl, dnsRecord.Type, dnsRecord.Value)
	rr, err := dns.NewRR(recordString)
	if err != nil {
		log.Printf("Error converting DnsRecord to dns.RR: %s\n", err)
		return nil
	}
	return &rr
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
