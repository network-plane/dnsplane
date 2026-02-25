// Package dnsrecordcache provides a simple in-memory cache
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsrecordcache

import (
	"dnsplane/cliutil"
	"dnsplane/dnsrecords"
	"errors"
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

var (
	ErrHelpRequested = errors.New("help requested")
	ErrInvalidArgs   = errors.New("invalid arguments")
)

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

// List returns the cache records without mutating them.
func List(cacheRecordsData []CacheRecord) []CacheRecord {
	return cacheRecordsData
}

// Remove a record from the cache

func Remove(fullCommand []string, cacheRecordsData []CacheRecord) ([]CacheRecord, []dnsrecords.Message, error) {
	messages := make([]dnsrecords.Message, 0)
	if len(fullCommand) == 0 || cliutil.IsHelpRequest(fullCommand) {
		return cacheRecordsData, usageCacheRemove(), ErrHelpRequested
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
		msgs := append([]dnsrecords.Message{{Level: dnsrecords.LevelWarn, Text: "No records found with the specified criteria."}}, usageCacheRemove()...)
		return cacheRecordsData, msgs, ErrInvalidArgs
	}

	if len(matchingIdx) > 1 && valueKey == "" {
		msgs := []dnsrecords.Message{{Level: dnsrecords.LevelWarn, Text: "Multiple records found. Please specify the type and value to remove a specific entry."}}
		for _, idx := range matchingIdx {
			record := cacheRecordsData[idx]
			msgs = append(msgs, dnsrecords.Message{Level: dnsrecords.LevelInfo, Text: fmt.Sprintf("- %s %s %s %d", record.DNSRecord.Name, record.DNSRecord.Type, record.DNSRecord.Value, record.DNSRecord.TTL)})
		}
		msgs = append(msgs, usageCacheRemove()...)
		return cacheRecordsData, msgs, ErrInvalidArgs
	}

	idx := matchingIdx[0]
	removed := cacheRecordsData[idx]
	cacheRecordsData = append(cacheRecordsData[:idx], cacheRecordsData[idx+1:]...)
	messages = append(messages, dnsrecords.Message{Level: dnsrecords.LevelInfo, Text: fmt.Sprintf("Removed: %s %s %s %d", removed.DNSRecord.Name, removed.DNSRecord.Type, removed.DNSRecord.Value, removed.DNSRecord.TTL)})
	return cacheRecordsData, messages, nil
}

func usageCacheRemove() []dnsrecords.Message {
	msgs := []dnsrecords.Message{
		{Level: dnsrecords.LevelInfo, Text: "Usage: cache remove <Name> [Type] [Value]"},
		{Level: dnsrecords.LevelInfo, Text: "Description: Remove a cache entry, optionally narrowing by record type and value."},
	}
	return append(msgs, cacheHelpHint())
}

func cacheHelpHint() dnsrecords.Message {
	return dnsrecords.Message{Level: dnsrecords.LevelInfo, Text: "Hint: append '?', 'help', or 'h' after the command to view this usage."}
}
