// Package dnsrecordcache provides a simple in-memory cache
package dnsrecordcache

import (
	"dnsresolver/cliutil"
	"dnsresolver/dnsrecords"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// CacheRecord holds the data for the cache records
type CacheRecord struct {
	DNSRecord dnsrecords.DNSRecord `json:"dns_record"`
	Expiry    time.Time            `json:"expiry,omitempty"`
	Timestamp time.Time            `json:"timestamp,omitempty"`
	LastQuery time.Time            `json:"last_query,omitempty"`
}

// Add a new record to the cache
func Add(cacheRecordsData []CacheRecord, record *dns.RR) []CacheRecord {
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
		DNSRecord: dnsrecords.DNSRecord{
			Name:  (*record).Header().Name,
			Type:  dns.TypeToString[(*record).Header().Rrtype],
			Value: value,
			TTL:   (*record).Header().Ttl,
		},
		Expiry:    time.Now().Add(time.Duration((*record).Header().Ttl) * time.Second),
		Timestamp: time.Now(),
	}

	// Check if the record already exists in the cache
	recordIndex := -1
	for i, existingRecord := range cacheRecordsData {
		if existingRecord.DNSRecord.Name == cacheRecord.DNSRecord.Name &&
			existingRecord.DNSRecord.Type == cacheRecord.DNSRecord.Type &&
			existingRecord.DNSRecord.Value == cacheRecord.DNSRecord.Value {
			recordIndex = i
			break
		}
	}

	// If the record exists in the cache, update its TTL, expiry, and last query, otherwise add it
	if recordIndex != -1 {
		cacheRecordsData[recordIndex].DNSRecord.TTL = cacheRecord.DNSRecord.TTL
		cacheRecordsData[recordIndex].Expiry = cacheRecord.Expiry
		cacheRecordsData[recordIndex].LastQuery = time.Now()
	} else {
		cacheRecord.LastQuery = time.Now()
		cacheRecordsData = append(cacheRecordsData, cacheRecord)
	}

	return cacheRecordsData
}

// List all records in the cache
func List(cacheRecordsData []CacheRecord) {
	fmt.Println("Cache Records:")
	for i, record := range cacheRecordsData {
		fmt.Printf("%d. %s %s %s %d\n", i+1, record.DNSRecord.Name, record.DNSRecord.Type, record.DNSRecord.Value, record.DNSRecord.TTL)
	}
}

// Remove a record from the cache

func Remove(fullCommand []string, cacheRecordsData []CacheRecord) []CacheRecord {
	if len(fullCommand) == 0 || cliutil.IsHelpRequest(fullCommand) {
		printCacheRemoveUsage()
		return cacheRecordsData
	}

	nameArg := strings.TrimSpace(fullCommand[0])
	nameKey := dnsrecords.NormalizeRecordNameKey(nameArg)

	typeArg := ""
	if len(fullCommand) >= 2 {
		typeArg = fullCommand[1]
	}
	typeKey := dnsrecords.NormalizeRecordType(typeArg)

	valueArg := ""
	if len(fullCommand) >= 3 {
		valueArg = strings.Join(fullCommand[2:], " ")
	}
	valueKey := ""
	if valueArg != "" {
		valueKey = dnsrecords.NormalizeRecordValueKey(typeKey, valueArg)
	}

	matchingIdx := make([]int, 0)
	for i, record := range cacheRecordsData {
		if dnsrecords.NormalizeRecordNameKey(record.DNSRecord.Name) != nameKey {
			continue
		}
		recordType := dnsrecords.NormalizeRecordType(record.DNSRecord.Type)
		if typeKey != "" && recordType != typeKey {
			continue
		}
		recordValueKey := dnsrecords.NormalizeRecordValueKey(recordType, record.DNSRecord.Value)
		if valueKey != "" && recordValueKey != valueKey {
			continue
		}
		matchingIdx = append(matchingIdx, i)
	}

	if len(matchingIdx) == 0 {
		fmt.Println("No records found with the specified criteria.")
		printCacheRemoveUsage()
		return cacheRecordsData
	}

	if len(matchingIdx) > 1 && valueKey == "" {
		fmt.Println("Multiple records found. Please specify the type and value to remove a specific entry.")
		for _, idx := range matchingIdx {
			record := cacheRecordsData[idx]
			fmt.Printf("- %s %s %s %d\n", record.DNSRecord.Name, record.DNSRecord.Type, record.DNSRecord.Value, record.DNSRecord.TTL)
		}
		printCacheRemoveUsage()
		return cacheRecordsData
	}

	idx := matchingIdx[0]
	removed := cacheRecordsData[idx]
	cacheRecordsData = append(cacheRecordsData[:idx], cacheRecordsData[idx+1:]...)
	fmt.Printf("Removed: %s %s %s %d\n", removed.DNSRecord.Name, removed.DNSRecord.Type, removed.DNSRecord.Value, removed.DNSRecord.TTL)
	return cacheRecordsData
}

func printCacheRemoveUsage() {
	fmt.Println("Usage: cache remove <Name> [Type] [Value]")
	fmt.Println("Description: Remove a cache entry, optionally narrowing by record type and value.")
	printHelpAliasesHint()
}

func printHelpAliasesHint() {
	fmt.Println("Hint: append '?', 'help', or 'h' after the command to view this usage.")
}
