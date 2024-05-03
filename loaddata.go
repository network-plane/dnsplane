package main

import (
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

func loadDNSRecords() []DNSRecord {
	type recordsType struct {
		Records []DNSRecord `json:"records"`
	}
	records := loadDataFromJSON[recordsType]("records.json")
	return records.Records
}

func loadSettings() DNSServerSettings {
	return loadDataFromJSON[DNSServerSettings]("dnsresolver.json")
}

func loadCacheRecords() []CacheRecord {
	type cacheType struct {
		Cache []CacheRecord `json:"cache"`
	}
	cache := loadDataFromJSON[cacheType]("cache.json")
	return cache.Cache
}
