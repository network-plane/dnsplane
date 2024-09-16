// Package data provides functions to load and save data from JSON files
package data

import (
	"dnsresolver/dnsrecordcache"
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
		Servers []dnsserver.DNSServer `json:"dnsservers"`
	}

	servers := LoadFromJSON[serversType]("dnsservers.json")
	return servers.Servers
}

// SaveDNSServers saves the DNS servers to the dnsservers.json file
func SaveDNSServers(dnsServers []dnsserver.DNSServer) error {
	type serversType struct {
		Servers []dnsserver.DNSServer `json:"dnsservers"`
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
func LoadCacheRecords() []dnsrecordcache.CacheRecord {
	type cacheType struct {
		Cache []dnsrecordcache.CacheRecord `json:"cache"`
	}
	cache := LoadFromJSON[cacheType]("dnscache.json")
	return cache.Cache
}

// SaveCacheRecords saves the cache records to the dnscache.json file
func SaveCacheRecords(cacheRecords []dnsrecordcache.CacheRecord) error {
	type cacheType struct {
		Cache []dnsrecordcache.CacheRecord `json:"cache"`
	}

	data := cacheType{Cache: cacheRecords}
	_ = data
	return SaveToJSON("dnscache.json", data)
}

// InitializeJSONFiles creates the JSON files if they don't exist
func InitializeJSONFiles() {
	CreateFileIfNotExists("dnsservers.json", `{"dnsservers":[{"address": "1.1.1.1","port": "53","active": false,"local_resolver": false,"adblocker": false }]}`)
	CreateFileIfNotExists("dnsrecords.json", `{"records": [{"name": "example.com.", "type": "A", "value": "93.184.216.34", "ttl": 3600, "last_query": "0001-01-01T00:00:00Z"}]}`)
	CreateFileIfNotExists("dnscache.json", `{"cache": [{"dns_record": {"name": "example.com","type": "A","value": "192.168.1.1","ttl": 3600,"added_on": "2024-05-01T12:00:00Z","updated_on": "2024-05-05T18:30:00Z","mac": "00:1A:2B:3C:4D:5E","last_query": "2024-05-07T15:45:00Z"},"expiry": "2024-05-10T12:00:00Z","timestamp": "2024-05-07T12:30:00Z","last_query": "2024-05-07T14:00:00Z"}]}`)
	CreateFileIfNotExists("dnsresolver.json", `{"fallback_server_ip": "192.168.178.21","fallback_server_port": "53","timeout": 2,"dns_port": "53","cache_records": true,"auto_build_ptr_from_a": true,"forward_ptr_queries": false,"file_locations": {"dnsserver_file": "dnsservers.json","dnsrecords_file": "dnsrecords.json","cache_file": "dnscache.json"}}`)
}

// CreateFileIfNotExists creates a file with the given filename and content if it does not exist
func CreateFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			log.Fatalf("Error creating %s: %s", filename, err)
		}
	}
}
