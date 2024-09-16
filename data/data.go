// Package data provides functions to load and save data from JSON files
package data

import (
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

//USAGE in FUNCTIONS:
//Load data
//dnsData := data.GetInstance()
//settings := dnsData.GetSettings()
//Use data here ...
//
//Stats
//dnsData.IncrementCacheHits()
//dnsData.IncrementTotalQueries()
//
//Update data
//
//dnsData.
//newSettings := data.DNSResolverSettings{...}
//dnsData.UpdateSettings(newSettings)

// DNSResolverData holds all the data for the DNS resolver
type DNSResolverData struct {
	Settings     DNSResolverSettings
	Stats        DNSStats
	DNSServers   []dnsserver.DNSServer
	Records      []dnsrecords.DNSRecord
	CacheRecords []dnsrecordcache.CacheRecord
	mu           sync.RWMutex
}

// DNSStats holds the data for the DNS statistics
type DNSStats struct {
	TotalQueries          int       `json:"total_queries"`
	TotalCacheHits        int       `json:"total_cache_hits"`
	TotalBlocks           int       `json:"total_blocks"`
	TotalQueriesForwarded int       `json:"total_queries_forwarded"`
	TotalQueriesAnswered  int       `json:"total_queries_answered"`
	ServerStartTime       time.Time `json:"server_start_time"`
}

// DNSResolverSettings holds DNS server settings
type DNSResolverSettings struct {
	FallbackServerIP   string        `json:"fallback_server_ip"`
	FallbackServerPort string        `json:"fallback_server_port"`
	Timeout            int           `json:"timeout"`
	DNSPort            string        `json:"dns_port"`
	MDNSPort           string        `json:"mdns_port"`
	RESTPort           string        `json:"rest_port"`
	CacheRecords       bool          `json:"cache_records"`
	AutoBuildPTRFromA  bool          `json:"auto_build_ptr_from_a"`
	ForwardPTRQueries  bool          `json:"forward_ptr_queries"`
	FileLocations      FileLocations `json:"file_locations"`
}

// FileLocations holds the file locations for the DNS server
type FileLocations struct {
	DNSServerFile  string `json:"dnsserver_file"`
	DNSRecordsFile string `json:"dnsrecords_file"`
	CacheFile      string `json:"cache_file"`
}

var instance *DNSResolverData

// GetInstance returns the singleton instance of DNSResolverData
func GetInstance() *DNSResolverData {
	if instance == nil {
		instance = &DNSResolverData{}
		instance.Initialize()
	}
	return instance
}

// Initialize loads all data from JSON files
func (d *DNSResolverData) Initialize() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Settings = LoadSettings()
	d.DNSServers = LoadDNSServers()
	d.Records = LoadDNSRecords()
	d.CacheRecords = LoadCacheRecords()
	d.Stats = DNSStats{ServerStartTime: time.Now()}
}

// GetResolverSettings returns the current DNS server settings
func (d *DNSResolverData) GetResolverSettings() DNSResolverSettings {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Settings
}

// UpdateSettings updates the DNS server settings
func (d *DNSResolverData) UpdateSettings(settings DNSResolverSettings) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Settings = settings
	SaveSettings(settings)
}

// GetStats returns the current DNS statistics
func (d *DNSResolverData) GetStats() DNSStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Stats
}

// UpdateStats updates the DNS statistics
func (d *DNSResolverData) UpdateStats(stats DNSStats) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats = stats
}

// GetServers returns the current DNS servers
func (d *DNSResolverData) GetServers() []dnsserver.DNSServer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DNSServers
}

// UpdateServers updates the DNS servers
func (d *DNSResolverData) UpdateServers(servers []dnsserver.DNSServer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.DNSServers = servers
	SaveDNSServers(servers)
}

// GetRecords returns the current DNS records
func (d *DNSResolverData) GetRecords() []dnsrecords.DNSRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Records
}

// UpdateRecords updates the DNS records
func (d *DNSResolverData) UpdateRecords(records []dnsrecords.DNSRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Records = records
	SaveDNSRecords(records)
}

// GetCacheRecords returns the current cache records
func (d *DNSResolverData) GetCacheRecords() []dnsrecordcache.CacheRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.CacheRecords
}

// UpdateCacheRecords updates the cache records
func (d *DNSResolverData) UpdateCacheRecords(records []dnsrecordcache.CacheRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.CacheRecords = records
	SaveCacheRecords(records)
}

// IncrementTotalQueries increments the total queries count
func (d *DNSResolverData) IncrementTotalQueries() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats.TotalQueries++
}

// IncrementCacheHits increments the cache hits count
func (d *DNSResolverData) IncrementCacheHits() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats.TotalCacheHits++
}

// IncrementTotalBlocks increments the total blocks count
func (d *DNSResolverData) IncrementTotalBlocks() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats.TotalBlocks++
}

// IncrementQueriesForwarded increments the queries forwarded count
func (d *DNSResolverData) IncrementQueriesForwarded() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats.TotalQueriesForwarded++
}

// IncrementQueriesAnswered increments the queries answered count
func (d *DNSResolverData) IncrementQueriesAnswered() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Stats.TotalQueriesAnswered++
}

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

// LoadSettings reads the dnsresolver.json file and returns the DNS server settings
func LoadSettings() DNSResolverSettings {
	return LoadFromJSON[DNSResolverSettings]("dnsresolver.json")
}

// SaveSettings saves the DNS server settings to the dnsresolver.json file
func SaveSettings(settings DNSResolverSettings) {
	if err := SaveToJSON("dnsresolver.json", settings); err != nil {
		log.Fatalf("Failed to save settings: %v", err)
	}
}
