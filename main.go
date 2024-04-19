package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/miekg/dns"
)

var (
	dnsServerSettings DNSServerSettings
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
		err := server.ListenAndServe()
		if err != nil {
			fmt.Println("Error starting server:", err)
			os.Exit(1)
		}
	}()

	// Interactive prompt running in the main goroutine
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		command, _ := reader.ReadString('\n')

		switch command = strings.TrimSpace(command); command {
		case "stats":
			showStats() // Implement this function based on your needs
		case "exit":
			fmt.Println("Shutting down the server.")
			server.Shutdown() // Gracefully shutdown the server
			return
		case "":
			continue
		case "?", "help", "h":
			fmt.Println("Available commands:")
			fmt.Println("  stats - Show server statistics")
			fmt.Println("  exit - Shutdown the server")
		default:
			fmt.Println("Unknown command:", command)
		}
	}
}

func showStats() {
	// Implement this function based on your needs
	fmt.Println("Stats:")
	fmt.Println("Total A Records:", len(getDNSRecords()))
	fmt.Println("Total DNS Servers:", len(getDNSServers()))

	fmt.Println("Total queries received:", "N/A")
	fmt.Println("Total queries answered:", "N/A")
	fmt.Println("Total queries forwarded:", "N/A")
}

func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = false

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
				processCacheRecord(question, cachedRecord, response)
			} else {
				handleDNSServers(question, dnsServers, fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)
			}
		}

	default:
		handleDNSServers(question, dnsServers, fmt.Sprintf("%s:%s", dnsServerSettings.FallbackServerIP, dnsServerSettings.FallbackServerPort), response)

	}
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
