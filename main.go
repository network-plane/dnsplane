package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func main() {
	initializeJsonFiles()

	dns.HandleFunc(".", handleRequest)

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
	dnsRecords := getDnsRecords()
	dnsServers := getDnsServers()
	fallbackServer := "192.168.178.21:53" // Choose a suitable fallback server
	cacheRecords, err := getCacheRecords()
	if err != nil {
		log.Println("Error getting cache records:", err)
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
			handleDnsServers(question, dnsServers, fallbackServer, response)
		}
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

func handleDnsServers(question dns.Question, dnsServers []string, fallbackServer string, response *dns.Msg) {
	answers := queryAllDnsServers(question, dnsServers)

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

func queryAllDnsServers(question dns.Question, dnsServers []string) <-chan *dns.Msg {
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

func queryDNS(domain, server string) (*dns.Msg, error) {
	client := &dns.Client{}
	msg := &dns.Msg{}

	msg.SetQuestion(domain, dns.TypeA)
	msg.RecursionDesired = true

	response, _, err := client.Exchange(msg, server)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func getDnsServers() []string {
	var servers Servers
	data, err := os.ReadFile("servers.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &servers)
	if err != nil {
		log.Fatal(err)
	}
	return servers.Servers
}

func getDnsRecords() []DnsRecord {
	var records Records
	data, err := os.ReadFile("records.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &records)
	if err != nil {
		log.Fatal(err)
	}
	return records.Records
}

func findRecord(records []DnsRecord, name, recordType string) *dns.RR {
	for _, record := range records {
		if record.Name == name && record.Type == recordType {
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

func getCacheRecords() ([]CacheRecord, error) {
	var cacheRecords []CacheRecord
	var records Records

	data, err := os.ReadFile("cache.json")
	if err != nil {
		return cacheRecords, err
	}

	err = json.Unmarshal(data, &records)
	if err != nil {
		return cacheRecords, err
	}

	cacheRecords = make([]CacheRecord, len(records.Records))
	for i, record := range records.Records {
		cacheRecords[i].DnsRecord = record
		cacheRecords[i].Timestamp = time.Now()
		cacheRecords[i].Expiry = record.LastQuery.Add(time.Duration(record.TTL) * time.Second)
	}
	return cacheRecords, nil
}

func saveCacheRecords(cacheRecords []CacheRecord) {
	records := Records{Records: make([]DnsRecord, len(cacheRecords))}
	for i, cacheRecord := range cacheRecords {
		records.Records[i] = cacheRecord.DnsRecord
		records.Records[i].TTL = uint32(cacheRecord.Expiry.Sub(cacheRecord.Timestamp).Seconds())
		records.Records[i].LastQuery = cacheRecord.LastQuery
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		log.Println("Error marshalling cache records:", err)
		return
	}
	err = os.WriteFile("cache.json", data, 0644)
	if err != nil {
		log.Println("Error saving cache records:", err)
	}
}

func findCacheRecord(cacheRecords []CacheRecord, name string, recordType string) *dns.RR {
	now := time.Now()
	for _, record := range cacheRecords {
		if record.DnsRecord.Name == name && record.DnsRecord.Type == recordType {
			if now.Before(record.Expiry) {
				remainingTTL := uint32(record.Expiry.Sub(now).Seconds())
				return dnsRecordToRR(&record.DnsRecord, remainingTTL)
			}
		}
	}
	return nil
}

func addToCache(cacheRecords []CacheRecord, record *dns.RR) []CacheRecord {
	var value string
	switch r := (*record).(type) {
	case *dns.A:
		value = r.A.String()
	case *dns.AAAA:
		value = r.AAAA.String()
	case *dns.CNAME:
		value = r.Target
	case *dns.MX:
		value = fmt.Sprintf("%d %s", r.Preference, r.Mx)
	case *dns.NS:
		value = r.Ns
	case *dns.SOA:
		value = fmt.Sprintf("%s %s %d %d %d %d %d", r.Ns, r.Mbox, r.Serial, r.Refresh, r.Retry, r.Expire, r.Minttl)
	case *dns.TXT:
		value = strings.Join(r.Txt, " ")
	default:
		value = (*record).String()
	}

	cacheRecord := CacheRecord{
		DnsRecord: DnsRecord{
			Name:  (*record).Header().Name,
			Type:  dns.TypeToString[(*record).Header().Rrtype],
			Value: value,
			TTL:   (*record).Header().Ttl,
		},
		Expiry:    time.Now().Add(time.Duration((*record).Header().Ttl) * time.Second),
		Timestamp: time.Now(), // Add this line
	}

	// Check if the record already exists in the cache
	recordIndex := -1
	for i, existingRecord := range cacheRecords {
		if existingRecord.DnsRecord.Name == cacheRecord.DnsRecord.Name &&
			existingRecord.DnsRecord.Type == cacheRecord.DnsRecord.Type &&
			existingRecord.DnsRecord.Value == cacheRecord.DnsRecord.Value {
			recordIndex = i
			break
		}
	}

	// If the record exists in the cache, update its TTL, expiry, and last query, otherwise add it
	if recordIndex != -1 {
		cacheRecords[recordIndex].DnsRecord.TTL = cacheRecord.DnsRecord.TTL
		cacheRecords[recordIndex].Expiry = cacheRecord.Expiry
		cacheRecords[recordIndex].LastQuery = time.Now()
	} else {
		cacheRecord.LastQuery = time.Now()
		cacheRecords = append(cacheRecords, cacheRecord)
	}

	saveCacheRecords(cacheRecords)
	return cacheRecords
}

func dnsRecordToRR(dnsRecord *DnsRecord, ttl uint32) *dns.RR {
	recordString := fmt.Sprintf("%s %d IN %s %s", dnsRecord.Name, ttl, dnsRecord.Type, dnsRecord.Value)
	rr, err := dns.NewRR(recordString)
	if err != nil {
		log.Printf("Error converting DnsRecord to dns.RR: %s\n", err)
		return nil
	}
	return &rr
}

func initializeJsonFiles() {
	createFileIfNotExists("servers.json", `{"servers": ["8.8.8.8:53", "8.8.4.4:53"]}`)
	createFileIfNotExists("records.json", `{"records": [{"Name": "example.com.", "Type": "A", "Value": "93.184.216.34", "TTL": 3600}]}`)
	createFileIfNotExists("cache.json", `{"records": []}`)
}

func createFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			log.Fatalf("Error creating %s: %s", filename, err)
		}
	}
}
