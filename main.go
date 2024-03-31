package main

import (
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

	log.Printf("Starting DNS server on %s\n", server.Addr)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
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

	if question.Qtype == dns.TypePTR {
		handlePTRQuestion(question, response)
		return
	}

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

	}
}

// convertReverseDNSToIP takes a reverse DNS lookup string and converts it back to an IP address.
func convertReverseDNSToIP(reverseDNS string) string {
	// Split the reverse DNS string by "."
	parts := strings.Split(reverseDNS, ".")

	// Check if the input is valid (should have at least 4 parts before "in-addr" and "arpa")
	if len(parts) < 6 {
		return "Invalid input"
	}

	// Extract the first four segments which represent the reversed IP address
	ipParts := parts[:4]

	// Reverse the order of the extracted segments
	for i, j := 0, len(ipParts)-1; i < j; i, j = i+1, j-1 {
		ipParts[i], ipParts[j] = ipParts[j], ipParts[i]
	}

	// Join the segments back together to form the original IP address
	return strings.Join(ipParts, ".")
}

// convertIPToReverseDNS takes an IP address and converts it to a reverse DNS lookup string.
func convertIPToReverseDNS(ip string) string {
	// Split the IP address into its segments
	parts := strings.Split(ip, ".")

	// Check if the input is a valid IPv4 address (should have exactly 4 parts)
	if len(parts) != 4 {
		return "Invalid IP address"
	}

	// Reverse the order of the IP segments
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	// Join the reversed segments and append the ".in-addr.arpa" domain
	reverseDNS := strings.Join(parts, ".") + ".in-addr.arpa"

	return reverseDNS
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

func findRecord(records []DNSRecord, lookupRecord, recordType string) *dns.RR {
	for _, record := range records {

		if recordType == "PTR" {
			if record.Value == lookupRecord {
				recordString := fmt.Sprintf("%s %d IN PTR %s.", convertIPToReverseDNS(lookupRecord), record.TTL, strings.TrimRight(record.Name, "."))
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
