// Package data provides functions to load and save data from JSON files
package data

import (
	"dnsresolver/cache"
	"dnsresolver/dnsrecords"
	"encoding/json"
	"log"
	"os"
)

// LoadFromJSON reads a JSON file and unmarshals it into a struct
func LoadFromJSON[T any](filePath string) T {
	var result T
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	return result
}

// SaveToJSON marshals data and saves it to a JSON file
func SaveToJSON[T any](filePath string, data T) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// LoadDNSServers reads the servers.json file and returns the list of DNS servers
func LoadDNSServers() []string {
	type serversType struct {
		Servers []string `json:"servers"`
	}

	servers := LoadFromJSON[serversType]("servers.json")
	return servers.Servers
}

// LoadDNSRecords reads the records.json file and returns the list of DNS records
func LoadDNSRecords() []dnsrecords.DNSRecord {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}
	records := LoadFromJSON[recordsType]("records.json")
	return records.Records
}

func saveDNSRecords(gDNSRecords []dnsrecords.DNSRecord) error {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}

	data := recordsType{Records: gDNSRecords}
	return SaveToJSON("records.json", data)
}

// LoadCacheRecords reads the cache.json file and returns the list of cache records
func LoadCacheRecords() []cache.Record {
	type cacheType struct {
		Cache []cache.Record `json:"cache"`
	}
	cache := LoadFromJSON[cacheType]("cache.json")
	return cache.Cache
}

// SaveCacheRecords saves the cache records to the cache.json file
func SaveCacheRecords(cacheRecords []cache.Record) {
	err := SaveToJSON("cache.json", cacheRecords)
	if err != nil {
		log.Println("Error saving cache records:", err)
	}
}
