package main

import (
	"dnsresolver/cache"
	"dnsresolver/dnsrecords"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func findCacheRecord(cacheRecords []cache.Record, name string, recordType string) *dns.RR {
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
	for _, record := range records {

		if record.Type == "PTR" || (recordType == "PTR" && dnsServerSettings.AutoBuildPTRFromA) {
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
