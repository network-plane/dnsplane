package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileName is the canonical name of the configuration file.
	FileName = "dnsplane.json"
	// systemConfigPath is the location checked last when resolving the config.
	systemConfigPath = "/etc/" + FileName
)

// RecordsSourceType is how the DNS records file is provided: local file, HTTP(S) URL, or git repo.
const (
	RecordsSourceFile = "file"
	RecordsSourceURL  = "url"
	RecordsSourceGit  = "git"
)

// RecordsSourceConfig describes where to load DNS records from (single source: file, URL, or git).
// When Type is "file", Location is the local path (read/write). When "url" or "git", Location is the URL and records are read-only; RefreshIntervalSeconds controls re-fetch interval.
type RecordsSourceConfig struct {
	Type                   string `json:"type"`                               // "file", "url", or "git"
	Location               string `json:"location"`                           // path, http(s) URL, or git repo URL
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds,omitempty"` // for url/git: how often to check for changes (seconds)
}

// FileLocations describes the JSON data files used by dnsplane.
// Records are always loaded from records_source (type "file", "url", or "git"); dnsservers and cache are paths.
type FileLocations struct {
	DNSServerFile string               `json:"dnsservers"`
	CacheFile     string               `json:"cache"`
	RecordsSource *RecordsSourceConfig `json:"records_source"` // required: type "file"|"url"|"git", location = path or URL
}

// DNSRecordSettings mirrors record handling settings persisted in the config.
type DNSRecordSettings struct {
	AutoBuildPTRFromA bool `json:"auto_build_ptr_from_a"`
	ForwardPTRQueries bool `json:"forward_ptr_queries"`
	AddUpdatesRecords bool `json:"add_updates_records,omitempty"`
}

// LogRotationMode is the log rotation strategy: "none", "size", or "time".
type LogRotationMode string

const (
	LogRotationNone LogRotationMode = "none"
	LogRotationSize LogRotationMode = "size"
	LogRotationTime LogRotationMode = "time"
)

// LogConfig holds logging directory, severity, and rotation settings.
type LogConfig struct {
	Dir            string          `json:"log_dir"`
	Severity       string          `json:"log_severity"`
	Rotation       LogRotationMode `json:"log_rotation"`
	RotationSizeMB int             `json:"log_rotation_size_mb"`
	RotationDays   int             `json:"log_rotation_time_days"`
}

// Config captures all persisted settings for dnsplane.
// Port/socket keys match CLI flags: --port, --apiport, --api, --server-socket, --server-tcp.
type Config struct {
	FallbackServerIP   string            `json:"fallback_server_ip"`
	FallbackServerPort string            `json:"fallback_server_port"`
	Timeout            int               `json:"timeout"`
	DNSPort            string            `json:"port"`
	RESTPort           string            `json:"apiport"`
	APIEnabled         bool              `json:"api"`
	CacheRecords       bool              `json:"cache_records"`
	FullStats          bool              `json:"full_stats"`
	FullStatsDir       string            `json:"full_stats_dir"`
	ClientSocketPath   string            `json:"server_socket"`
	ClientTCPAddress   string            `json:"server_tcp"`
	FileLocations      FileLocations     `json:"file_locations"`
	DNSRecordSettings  DNSRecordSettings `json:"DNSRecordSettings"`
	Log                LogConfig         `json:"log"`
	// AdblockListFiles is a list of paths to adblock list files (e.g. hosts-style). Loaded in order at startup and merged into a single block list.
	AdblockListFiles []string `json:"adblock_list_files,omitempty"`
}

// Loaded contains the configuration together with metadata about the source file.
type Loaded struct {
	Path    string
	Created bool
	Config  Config
}

// Load resolves the dnsplane configuration file, creating a default one if
// necessary, and returns the parsed configuration alongside metadata.
func Load() (*Loaded, error) {
	candidates, err := candidatePaths()
	if err != nil {
		return nil, err
	}

	for _, path := range candidates {
		cfg, err := readConfig(path)
		if err == nil {
			cfg.applyDefaults(filepath.Dir(path))
			return &Loaded{Path: path, Config: *cfg}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config: failed to read %s: %w", path, err)
		}
	}

	defaultDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("config: determine working directory: %w", err)
	}

	defaultPath := filepath.Join(defaultDir, FileName)
	cfg := defaultConfig(defaultDir)
	if err := writeConfig(defaultPath, cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults(defaultDir)
	return &Loaded{Path: defaultPath, Created: true, Config: *cfg}, nil
}

// resolveConfigPath returns the config file path. If path is a directory (ends
// with /, exists as dir, or path has no extension), returns path/FileName;
// otherwise returns path as the config file path.
func resolveConfigPath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return "", fmt.Errorf("config: path is empty")
	}
	isDir := strings.HasSuffix(path, string(filepath.Separator))
	if !isDir {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			isDir = true
		} else if !strings.Contains(filepath.Base(path), ".") {
			isDir = true
		}
	}
	if isDir {
		path = strings.TrimSuffix(path, string(filepath.Separator))
		return filepath.Join(path, FileName), nil
	}
	return path, nil
}

// LoadFromPath loads configuration from the given path, or creates a default
// config at that path if the file does not exist. Path may be a directory
// (then config is path/dnsplane.json) or a file path (then that file is used).
func LoadFromPath(path string) (*Loaded, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config: path is empty")
	}
	configPath, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}
	cfg, err := readConfig(configPath)
	if err == nil {
		cfg.applyDefaults(filepath.Dir(configPath))
		return &Loaded{Path: configPath, Config: *cfg}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: failed to read %s: %w", configPath, err)
	}
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("config: ensure config directory %s: %w", dir, err)
	}
	defaultCfg := defaultConfig(dir)
	if err := writeConfig(configPath, defaultCfg); err != nil {
		return nil, err
	}
	defaultCfg.applyDefaults(dir)
	return &Loaded{Path: configPath, Created: true, Config: *defaultCfg}, nil
}

// Read loads and normalises configuration from the specified path without
// searching other locations.
func Read(path string) (*Config, error) {
	cfg, err := readConfig(path)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults(filepath.Dir(path))
	return cfg, nil
}

// Save writes the supplied configuration back to the given path.
func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: ensure config directory %s: %w", dir, err)
	}
	cfg.applyDefaults(dir)
	return writeConfig(path, &cfg)
}

// Normalize ensures derived fields like file paths are populated for the
// provided configuration relative to the supplied directory.
func (c *Config) Normalize(configDir string) {
	c.applyDefaults(configDir)
}

func candidatePaths() ([]string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("config: determine executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	var paths []string
	paths = appendIfMissing(paths, filepath.Join(execDir, FileName))

	if userPath, err := userConfigPath(); err == nil && userPath != "" {
		paths = appendIfMissing(paths, userPath)
	}

	paths = appendIfMissing(paths, systemConfigPath)
	return paths, nil
}

func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: determine user config dir: %w", err)
	}
	return filepath.Join(dir, "dnsplane", FileName), nil
}

func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("config: file %s is empty", path)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if legacy := extractLegacyRecordSettings(data); legacy != nil {
		cfg.DNSRecordSettings = *legacy
	}
	return &cfg, nil
}

func writeConfig(path string, cfg *Config) error {
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal config: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

func defaultConfig(baseDir string) *Config {
	logDir := "/var/log/dnsplane"
	if !isSystemConfigDir(baseDir) {
		logDir = filepath.Join(baseDir, "log")
	}
	return &Config{
		FallbackServerIP:   "1.1.1.1",
		FallbackServerPort: "53",
		Timeout:            2,
		DNSPort:            "53",
		RESTPort:           "8080",
		APIEnabled:         false,
		CacheRecords:       true,
		FullStats:          false,
		FullStatsDir:       filepath.Join(baseDir, "fullstats"),
		ClientSocketPath:   defaultSocketPath(),
		ClientTCPAddress:   "0.0.0.0:8053",
		FileLocations: FileLocations{
			DNSServerFile: filepath.Join(baseDir, "dnsservers.json"),
			CacheFile:     filepath.Join(baseDir, "dnscache.json"),
			RecordsSource: &RecordsSourceConfig{Type: RecordsSourceFile, Location: filepath.Join(baseDir, "dnsrecords.json")},
		},
		DNSRecordSettings: DNSRecordSettings{
			AutoBuildPTRFromA: true,
			ForwardPTRQueries: false,
		},
		Log: LogConfig{
			Dir:            logDir,
			Severity:       "none",
			Rotation:       LogRotationSize,
			RotationSizeMB: 100,
			RotationDays:   7,
		},
		AdblockListFiles: nil,
	}
}

func (c *Config) applyDefaults(configDir string) {
	if c.FallbackServerIP == "" {
		c.FallbackServerIP = "1.1.1.1"
	}
	if c.FallbackServerPort == "" {
		c.FallbackServerPort = "53"
	}
	if c.DNSPort == "" {
		c.DNSPort = "53"
	}
	if c.RESTPort == "" {
		c.RESTPort = "8080"
	}
	if c.ClientSocketPath == "" {
		c.ClientSocketPath = defaultSocketPath()
	}
	if c.ClientTCPAddress == "" {
		c.ClientTCPAddress = "0.0.0.0:8053"
	}
	if c.FullStatsDir == "" {
		c.FullStatsDir = filepath.Join(configDir, "fullstats")
	} else {
		c.FullStatsDir = ensureAbsolutePath(configDir, c.FullStatsDir, "fullstats")
	}

	c.FileLocations.DNSServerFile = ensureAbsolutePath(configDir, c.FileLocations.DNSServerFile, "dnsservers.json")
	c.FileLocations.CacheFile = ensureAbsolutePath(configDir, c.FileLocations.CacheFile, "dnscache.json")
	rs := c.FileLocations.RecordsSource
	if rs == nil {
		rs = &RecordsSourceConfig{Type: RecordsSourceFile, Location: filepath.Join(configDir, "dnsrecords.json")}
		c.FileLocations.RecordsSource = rs
	}
	if rs.Type == RecordsSourceURL || rs.Type == RecordsSourceGit {
		if rs.RefreshIntervalSeconds <= 0 {
			rs.RefreshIntervalSeconds = 60
		}
	} else {
		// type "file" or empty: treat as file, make location absolute
		rs.Type = RecordsSourceFile
		rs.Location = ensureAbsolutePath(configDir, rs.Location, "dnsrecords.json")
	}

	if c.Log.Dir == "" {
		if isSystemConfigDir(configDir) {
			c.Log.Dir = "/var/log/dnsplane"
		} else {
			c.Log.Dir = filepath.Join(configDir, "log")
		}
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "none"
	}
	// "none" means logging disabled: no log files created
	if c.Log.Rotation == "" {
		c.Log.Rotation = LogRotationSize
	}
	if c.Log.RotationSizeMB <= 0 {
		c.Log.RotationSizeMB = 100
	}
	if c.Log.RotationDays <= 0 {
		c.Log.RotationDays = 7
	}
}

func appendIfMissing(paths []string, candidate string) []string {
	for _, existing := range paths {
		if existing == candidate {
			return paths
		}
	}
	return append(paths, candidate)
}

func ensureAbsolutePath(configDir, value, fallbackName string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filepath.Join(configDir, fallbackName)
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(configDir, value)
}

// isSystemConfigDir returns true when configDir is the system config location (e.g. /etc or /etc/dnsplane),
// so log dir and other defaults can use system paths like /var/log/dnsplane.
func isSystemConfigDir(configDir string) bool {
	clean := filepath.Clean(configDir)
	return clean == "/etc" || strings.HasPrefix(clean, "/etc"+string(filepath.Separator))
}

func defaultSocketPath() string {
	if runningAsRoot() {
		return filepath.Join(os.TempDir(), "dnsplane.socket")
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "dnsplane.socket")
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "dnsplane", "dnsplane.socket")
	}
	return filepath.Join(os.TempDir(), "dnsplane.socket")
}

// DefaultClientSocketPath returns the default UNIX socket path for the TUI client and server.
// When running as non-root it uses XDG_RUNTIME_DIR or the user config dir so each user has their own socket.
func DefaultClientSocketPath() string {
	return defaultSocketPath()
}

func extractLegacyRecordSettings(data []byte) *DNSRecordSettings {
	type legacy struct {
		DNSRecordSettings *DNSRecordSettings `json:"DNSRecordSettings"`
	}
	var l legacy
	if err := json.Unmarshal(data, &l); err != nil {
		return nil
	}
	return l.DNSRecordSettings
}

// UnmarshalJSON reads FileLocations, accepting legacy keys (dnsserver_file, dnsrecords_file, cache_file).
// If records_source is missing but dnsrecords/dnsrecords_file is set, records_source is set to type "file" at that path.
func (fl *FileLocations) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	getStr := func(keys ...string) string {
		for _, k := range keys {
			if r, ok := raw[k]; ok && len(r) > 0 {
				var s string
				if json.Unmarshal(r, &s) == nil {
					return s
				}
			}
		}
		return ""
	}
	fl.DNSServerFile = getStr("dnsservers", "dnsserver_file")
	fl.CacheFile = getStr("cache", "cache_file")
	if r, ok := raw["records_source"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &fl.RecordsSource); err != nil {
			return err
		}
	}
	// Legacy: no records_source but dnsrecords path present → treat as file source
	if fl.RecordsSource == nil {
		legacyPath := getStr("dnsrecords", "dnsrecords_file")
		if legacyPath != "" {
			fl.RecordsSource = &RecordsSourceConfig{Type: RecordsSourceFile, Location: legacyPath}
		}
	}
	return nil
}

// UnmarshalJSON reads Config, accepting legacy keys (dns_port, rest_port, api_enabled, client_socket_path, client_tcp_address).
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	getStr := func(keys ...string) string {
		for _, k := range keys {
			if r, ok := raw[k]; ok && len(r) > 0 {
				var s string
				if json.Unmarshal(r, &s) == nil {
					return s
				}
			}
		}
		return ""
	}
	getBool := func(keys ...string) (bool, bool) {
		for _, k := range keys {
			if r, ok := raw[k]; ok && len(r) > 0 {
				var b bool
				if json.Unmarshal(r, &b) == nil {
					return b, true
				}
			}
		}
		return false, false
	}
	// Unmarshal all fields; for renamed ones we accept both new and old keys via getStr/getBool.
	if r, ok := raw["fallback_server_ip"]; ok {
		_ = json.Unmarshal(r, &c.FallbackServerIP)
	}
	if r, ok := raw["fallback_server_port"]; ok {
		_ = json.Unmarshal(r, &c.FallbackServerPort)
	}
	if r, ok := raw["timeout"]; ok {
		_ = json.Unmarshal(r, &c.Timeout)
	}
	c.DNSPort = getStr("port", "dns_port")
	c.RESTPort = getStr("apiport", "rest_port")
	if b, ok := getBool("api", "api_enabled"); ok {
		c.APIEnabled = b
	}
	if r, ok := raw["cache_records"]; ok {
		_ = json.Unmarshal(r, &c.CacheRecords)
	}
	if r, ok := raw["full_stats"]; ok {
		_ = json.Unmarshal(r, &c.FullStats)
	}
	if r, ok := raw["full_stats_dir"]; ok {
		_ = json.Unmarshal(r, &c.FullStatsDir)
	}
	c.ClientSocketPath = getStr("server_socket", "client_socket_path")
	c.ClientTCPAddress = getStr("server_tcp", "client_tcp_address")
	if r, ok := raw["file_locations"]; ok {
		_ = json.Unmarshal(r, &c.FileLocations)
	}
	if r, ok := raw["DNSRecordSettings"]; ok {
		_ = json.Unmarshal(r, &c.DNSRecordSettings)
	}
	if r, ok := raw["log"]; ok {
		_ = json.Unmarshal(r, &c.Log)
	}
	// If any renamed field was missing, standard unmarshal would leave zero value; apply defaults later via applyDefaults
	return nil
}
