package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/miekg/dns"
)

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
	dnsRecords := loadDNSRecords()
	dnsServers := loadDNSServers()
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
	dnsRecords := loadDNSRecords()
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

func dnsRecordToRR(dnsRecord *DNSRecord, ttl uint32) *dns.RR {
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
