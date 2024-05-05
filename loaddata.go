package main

import (
	"dnsresolver/cache"
	"dnsresolver/dnsrecords"
	"encoding/json"
	"log"
	"os"
)

func loadDataFromJSON[T any](filePath string) T {
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

func loadDNSServers() []string {
	type serversType struct {
		Servers []string `json:"servers"`
	}
	servers := loadDataFromJSON[serversType]("servers.json")
	return servers.Servers
}

func loadDNSRecords() []dnsrecords.DNSRecord {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}
	records := loadDataFromJSON[recordsType]("records.json")
	return records.Records
}

func saveDNSRecords() error {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}

	// Wrap the global dnsRecords in a struct to match the desired JSON format
	data := recordsType{Records: gDNSRecords}

	// Open the file for writing
	file, err := os.Create("records.json")
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a JSON encoder and write the data to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Optional: format the JSON output with indentation
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}

func loadSettings() DNSServerSettings {
	return loadDataFromJSON[DNSServerSettings]("dnsresolver.json")
}

func loadCacheRecords() []cache.Record {
	type cacheType struct {
		Cache []cache.Record `json:"cache"`
	}
	cache := loadDataFromJSON[cacheType]("cache.json")
	return cache.Cache
}

func saveCacheRecords(cacheRecords []cache.Record) {
	for i, cacheRecord := range cacheRecords {
		gDNSRecords[i] = cacheRecord.DNSRecord
		gDNSRecords[i].TTL = uint32(cacheRecord.Expiry.Sub(cacheRecord.Timestamp).Seconds())
		gDNSRecords[i].LastQuery = cacheRecord.LastQuery
	}
	data, err := json.MarshalIndent(gDNSRecords, "", "  ")
	if err != nil {
		log.Println("Error marshalling cache records:", err)
		return
	}
	err = os.WriteFile("cache.json", data, 0644)
	if err != nil {
		log.Println("Error saving cache records:", err)
	}
}
