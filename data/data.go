// Package data provides functions to load and save data from JSON files
package data

import (
	"dnsresolver/cache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
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

// LoadDNSServers reads the dnsservers.json file and returns the list of DNS servers
func LoadDNSServers() []dnsserver.DNSServer {
	type serversType struct {
		Servers []dnsserver.DNSServer
	}

	servers := LoadFromJSON[serversType]("dnsservers.json")
	return servers.Servers
}

// SaveDNSServer saves the DNS servers to the dnsservers.json file
func SaveDNSServer(dnsServers []dnsserver.DNSServer) error {
	type serversType struct {
		Servers []dnsserver.DNSServer
	}

	data := serversType{Servers: dnsServers}
	return SaveToJSON("dnsservers.json", data)
}

// LoadDNSRecords reads the dnsrecords.json file and returns the list of DNS records
func LoadDNSRecords() []dnsrecords.DNSRecord {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}
	records := LoadFromJSON[recordsType]("dnsrecords.json")
	return records.Records
}

// SaveDNSRecords saves the DNS records to the dnsrecords.json file
func SaveDNSRecords(gDNSRecords []dnsrecords.DNSRecord) error {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}

	data := recordsType{Records: gDNSRecords}
	return SaveToJSON("dnsrecords.json", data)
}

// LoadCacheRecords reads the dnscache.json file and returns the list of cache records
func LoadCacheRecords() []cache.Record {
	type cacheType struct {
		Cache []cache.Record `json:"cache"`
	}
	cache := LoadFromJSON[cacheType]("dnscache.json")
	return cache.Cache
}

// SaveCacheRecords saves the cache records to the dnscache.json file
func SaveCacheRecords(cacheRecords []cache.Record) {
	err := SaveToJSON("dnscache.json", cacheRecords)
	if err != nil {
		log.Println("Error saving cache records:", err)
	}
}
