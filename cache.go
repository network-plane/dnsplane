package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func saveCacheRecords(cacheRecords []CacheRecord) {
	for i, cacheRecord := range cacheRecords {
		dnsRecords[i] = cacheRecord.DNSRecord
		dnsRecords[i].TTL = uint32(cacheRecord.Expiry.Sub(cacheRecord.Timestamp).Seconds())
		dnsRecords[i].LastQuery = cacheRecord.LastQuery
	}
	data, err := json.MarshalIndent(dnsRecords, "", "  ")
	if err != nil {
		log.Println("Error marshalling cache records:", err)
		return
	}
	err = os.WriteFile("cache.json", data, 0644)
	if err != nil {
		log.Println("Error saving cache records:", err)
	}
}

func addToCache(cacheRecords []CacheRecord, record *dns.RR) []CacheRecord {

	if !dnsServerSettings.CacheRecords {
		return cacheRecords
	}

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
		DNSRecord: DNSRecord{
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
		if existingRecord.DNSRecord.Name == cacheRecord.DNSRecord.Name &&
			existingRecord.DNSRecord.Type == cacheRecord.DNSRecord.Type &&
			existingRecord.DNSRecord.Value == cacheRecord.DNSRecord.Value {
			recordIndex = i
			break
		}
	}

	// If the record exists in the cache, update its TTL, expiry, and last query, otherwise add it
	if recordIndex != -1 {
		cacheRecords[recordIndex].DNSRecord.TTL = cacheRecord.DNSRecord.TTL
		cacheRecords[recordIndex].Expiry = cacheRecord.Expiry
		cacheRecords[recordIndex].LastQuery = time.Now()
	} else {
		cacheRecord.LastQuery = time.Now()
		cacheRecords = append(cacheRecords, cacheRecord)
	}

	saveCacheRecords(cacheRecords)
	return cacheRecords
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
