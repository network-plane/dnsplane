// Package data provides functions to load and save data from JSON files
package data

import (
	"dnsplane/adblock"
	"dnsplane/config"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
	"dnsplane/dnsservers"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	DNSServers   []dnsservers.DNSServer
	DNSRecords   []dnsrecords.DNSRecord
	CacheRecords []dnsrecordcache.CacheRecord
	BlockList    *adblock.BlockList
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

// DNSResolverSettings is an alias to the configuration structure.
type DNSResolverSettings = config.Config

// DNSRecordSettings is an alias to the record settings persisted in config.
type DNSRecordSettings = config.DNSRecordSettings

// FileLocations is an alias to config-defined file locations.
type FileLocations = config.FileLocations

var (
	instance      *DNSResolverData
	configStateMu sync.RWMutex
	configState   *config.Loaded
)

// SetConfig stores the loaded configuration for subsequent data operations.
func SetConfig(loaded *config.Loaded) {
	if loaded == nil {
		log.Fatalf("data: configuration not provided")
	}
	configStateMu.Lock()
	defer configStateMu.Unlock()
	clone := *loaded
	configState = &clone
}

func currentConfig() config.Loaded {
	configStateMu.RLock()
	defer configStateMu.RUnlock()
	if configState == nil {
		log.Fatalf("data: configuration not initialised; call data.SetConfig before use")
	}
	return *configState
}

func updateStoredConfig(cfgPath string, cfg config.Config) {
	configStateMu.Lock()
	defer configStateMu.Unlock()
	if configState == nil {
		configState = &config.Loaded{}
	}
	configState.Path = cfgPath
	configState.Config = cfg
}

// For Removal in the future

// CacheRecord holds the data for the cache records
type CacheRecord struct {
	DNSRecord dnsrecords.DNSRecord `json:"dns_record"`
	Expiry    time.Time            `json:"expiry,omitempty"`
	Timestamp time.Time            `json:"timestamp,omitempty"`
	LastQuery time.Time            `json:"last_query,omitempty"`
}

// GetInstance returns the singleton instance of DNSResolverData
func GetInstance() *DNSResolverData {
	if instance == nil {
		instance = &DNSResolverData{}
		if err := instance.Initialize(); err != nil {
			log.Fatalf("data: failed to initialise resolver data: %v", err)
		}
	}
	return instance
}

// Initialize loads all data from JSON files
func (d *DNSResolverData) Initialize() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	cfg := currentConfig()
	d.Settings = cfg.Config

	servers, err := LoadDNSServers()
	if err != nil {
		return fmt.Errorf("load dns servers: %w", err)
	}
	records, err := LoadDNSRecords()
	if err != nil {
		return fmt.Errorf("load dns records: %w", err)
	}
	cache, err := LoadCacheRecords()
	if err != nil {
		return fmt.Errorf("load cache records: %w", err)
	}

	d.DNSServers = servers
	d.DNSRecords = records
	d.CacheRecords = cache
	d.BlockList = adblock.NewBlockList()
	d.Stats = DNSStats{ServerStartTime: time.Now()}
	return nil
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

// UpdateSettingsInMemory replaces the settings without persisting them to disk.
func (d *DNSResolverData) UpdateSettingsInMemory(settings DNSResolverSettings) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Settings = settings
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
func (d *DNSResolverData) GetServers() []dnsservers.DNSServer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return copyDNSServers(d.DNSServers)
}

// UpdateServers updates the DNS servers
func (d *DNSResolverData) UpdateServers(servers []dnsservers.DNSServer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.DNSServers = servers
	err := SaveDNSServers(servers)
	if err != nil {
		fmt.Println("Failed to save cache records:", err)
	}
}

// GetRecords returns the current DNS records
func (d *DNSResolverData) GetRecords() []dnsrecords.DNSRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return copyDNSRecords(d.DNSRecords)
}

// UpdateRecords updates the DNS records
func (d *DNSResolverData) UpdateRecords(records []dnsrecords.DNSRecord) {
	d.storeRecords(records, true)
}

// UpdateRecordsInMemory replaces DNS records without writing to disk.
func (d *DNSResolverData) UpdateRecordsInMemory(records []dnsrecords.DNSRecord) {
	d.storeRecords(records, false)
}

// GetCacheRecords returns the current cache records
func (d *DNSResolverData) GetCacheRecords() []dnsrecordcache.CacheRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return copyCacheRecords(d.CacheRecords)
}

// UpdateCacheRecords updates the cache records
func (d *DNSResolverData) UpdateCacheRecords(records []dnsrecordcache.CacheRecord) {
	d.storeCacheRecords(records, true)
}

// UpdateCacheRecordsInMemory replaces cache records without writing to disk.
func (d *DNSResolverData) UpdateCacheRecordsInMemory(records []dnsrecordcache.CacheRecord) {
	d.storeCacheRecords(records, false)
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

// GetBlockList returns the adblock list
func (d *DNSResolverData) GetBlockList() *adblock.BlockList {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.BlockList
}

// LoadFromJSON reads a JSON file and unmarshals it into a struct
func LoadFromJSON[T any](filePath string) (T, error) {
	var result T
	data, err := os.ReadFile(filePath)
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}

	return result, nil
}

// SaveToJSON marshals data and saves it to a JSON file
func SaveToJSON[T any](filePath string, data T) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
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
func LoadDNSServers() ([]dnsservers.DNSServer, error) {
	type serversType struct {
		Servers []dnsservers.DNSServer `json:"dnsservers"`
	}

	paths := currentConfig().Config.FileLocations
	servers, err := LoadFromJSON[serversType](paths.DNSServerFile)
	if err != nil {
		return nil, err
	}
	return servers.Servers, nil
}

// SaveDNSServers saves the DNS servers to the dnsservers.json file
func SaveDNSServers(dnsServers []dnsservers.DNSServer) error {
	type serversType struct {
		Servers []dnsservers.DNSServer `json:"dnsservers"`
	}

	data := serversType{Servers: dnsServers}
	paths := currentConfig().Config.FileLocations
	return SaveToJSON(paths.DNSServerFile, data)
}

// LoadDNSRecords reads the dnsrecords.json file and returns the list of DNS records
func LoadDNSRecords() ([]dnsrecords.DNSRecord, error) {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}
	paths := currentConfig().Config.FileLocations
	records, err := LoadFromJSON[recordsType](paths.DNSRecordsFile)
	if err != nil {
		return nil, err
	}
	return records.Records, nil
}

// SaveDNSRecords saves the DNS records to the dnsrecords.json file
func SaveDNSRecords(gDNSRecords []dnsrecords.DNSRecord) error {
	type recordsType struct {
		Records []dnsrecords.DNSRecord `json:"records"`
	}

	data := recordsType{Records: gDNSRecords}
	paths := currentConfig().Config.FileLocations
	return SaveToJSON(paths.DNSRecordsFile, data)
}

// LoadCacheRecords reads the dnscache.json file and returns the list of cache records
func LoadCacheRecords() ([]dnsrecordcache.CacheRecord, error) {
	type cacheType struct {
		Cache []dnsrecordcache.CacheRecord `json:"cache"`
	}
	paths := currentConfig().Config.FileLocations
	cache, err := LoadFromJSON[cacheType](paths.CacheFile)
	if err != nil {
		return nil, err
	}
	return cache.Cache, nil
}

// SaveCacheRecords saves the cache records to the dnscache.json file
func SaveCacheRecords(cacheRecords []dnsrecordcache.CacheRecord) error {
	type cacheType struct {
		Cache []dnsrecordcache.CacheRecord `json:"cache"`
	}

	data := cacheType{Cache: cacheRecords}
	_ = data
	paths := currentConfig().Config.FileLocations
	return SaveToJSON(paths.CacheFile, data)
}

func (d *DNSResolverData) storeRecords(records []dnsrecords.DNSRecord, persist bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.DNSRecords = records
	if persist {
		if err := SaveDNSRecords(records); err != nil {
			fmt.Println("Failed to save DNS records:", err)
		}
	}
}

func (d *DNSResolverData) storeCacheRecords(records []dnsrecordcache.CacheRecord, persist bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.CacheRecords = records
	if persist {
		if err := SaveCacheRecords(records); err != nil {
			fmt.Println("Failed to save cache records:", err)
		}
	}
}

func copyDNSRecords(src []dnsrecords.DNSRecord) []dnsrecords.DNSRecord {
	if len(src) == 0 {
		return nil
	}
	dst := make([]dnsrecords.DNSRecord, len(src))
	copy(dst, src)
	return dst
}

func copyDNSServers(src []dnsservers.DNSServer) []dnsservers.DNSServer {
	if len(src) == 0 {
		return nil
	}
	dst := make([]dnsservers.DNSServer, len(src))
	copy(dst, src)
	return dst
}

func copyCacheRecords(src []dnsrecordcache.CacheRecord) []dnsrecordcache.CacheRecord {
	if len(src) == 0 {
		return nil
	}
	dst := make([]dnsrecordcache.CacheRecord, len(src))
	copy(dst, src)
	return dst
}

// InitializeJSONFiles creates the JSON files if they don't exist
func InitializeJSONFiles() {
	paths := currentConfig().Config.FileLocations
	CreateFileIfNotExists(paths.DNSServerFile, `{"dnsservers":[{"address": "1.1.1.1","port": "53","active": false,"local_resolver": false,"adblocker": false }]}`)
	CreateFileIfNotExists(paths.DNSRecordsFile, `{"records": [{"name": "example.com.", "type": "A", "value": "93.184.216.34", "ttl": 3600, "last_query": "0001-01-01T00:00:00Z"}]}`)
	CreateFileIfNotExists(paths.CacheFile, `{"cache": [{"dns_record": {"name": "example.com","type": "A","value": "192.168.1.1","ttl": 3600,"added_on": "2024-05-01T12:00:00Z","updated_on": "2024-05-05T18:30:00Z","mac": "00:1A:2B:3C:4D:5E","last_query": "2024-05-07T15:45:00Z"},"expiry": "2024-05-10T12:00:00Z","timestamp": "2024-05-07T12:30:00Z","last_query": "2024-05-07T14:00:00Z"}]}`)
}

// CreateFileIfNotExists creates a file with the given filename and content if it does not exist
func CreateFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); err == nil {
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Error checking %s: %s", filename, err)
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		log.Fatalf("Error creating directory for %s: %s", filename, err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		log.Fatalf("Error creating %s: %s", filename, err)
	}
}

// LoadSettings reads the dnsplane.json file and returns the DNS server settings
func LoadSettings() DNSResolverSettings {
	info := currentConfig()
	cfg, err := config.Read(info.Path)
	if err != nil {
		log.Fatalf("Failed to read settings: %v", err)
	}
	updateStoredConfig(info.Path, *cfg)
	return *cfg
}

// SaveSettings saves the DNS server settings to the dnsplane.json file
func SaveSettings(settings DNSResolverSettings) {
	info := currentConfig()
	if err := config.Save(info.Path, settings); err != nil {
		log.Fatalf("Failed to save settings: %v", err)
	}
	updateStoredConfig(info.Path, settings)
}
