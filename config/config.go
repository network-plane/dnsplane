// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
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
	FallbackServerIP   string `json:"fallback_server_ip"`
	FallbackServerPort string `json:"fallback_server_port"`
	Timeout            int    `json:"timeout"`
	DNSPort            string `json:"port"`
	RESTPort           string `json:"apiport"`
	APIEnabled         bool   `json:"api"`
	// APIAuthToken when non-empty requires Authorization: Bearer <token> or X-API-Token for all routes except GET/HEAD /health and /ready.
	APIAuthToken string `json:"api_auth_token,omitempty"`
	// DNSBind is the IP address to bind for DNS UDP/TCP listeners (e.g. "127.0.0.1"). Empty binds all interfaces.
	DNSBind string `json:"dns_bind,omitempty"`
	// APIBind is the IP address to bind for the REST API (e.g. "127.0.0.1"). Empty binds all interfaces.
	APIBind string `json:"api_bind,omitempty"`
	// APITLSCertFile and APITLSKeyFile when both non-empty enable HTTPS for the REST API (ListenAndServeTLS).
	APITLSCertFile string `json:"api_tls_cert,omitempty"`
	APITLSKeyFile  string `json:"api_tls_key,omitempty"`
	// APIRateLimitPerIP is max sustained HTTP requests per second per client IP (0 = disabled). Uses token bucket.
	APIRateLimitPerIP float64 `json:"api_rate_limit_rps,omitempty"`
	// APIRateLimitBurst is max burst size when API rate limiting is enabled (default 20).
	APIRateLimitBurst int `json:"api_rate_limit_burst,omitempty"`
	// DNSRateLimitPerIP is max DNS queries per second per client IP UDP/TCP (0 = disabled).
	DNSRateLimitPerIP float64 `json:"dns_rate_limit_rps,omitempty"`
	// DNSRateLimitBurst is burst for DNS rate limiting (default 50).
	DNSRateLimitBurst int `json:"dns_rate_limit_burst,omitempty"`
	// DNSAmplificationMaxRatio caps packed response size vs packed request (0 = disabled). Default 100 when >0 check enabled.
	DNSAmplificationMaxRatio int `json:"dns_amplification_max_ratio,omitempty"`
	// FallbackServerTransport is udp, tcp, dot, or doh for the fallback resolver (default udp).
	FallbackServerTransport string `json:"fallback_server_transport,omitempty"`

	// Inbound DoT (DNS over TLS) server — tcp-tls listener separate from plain DNS port.
	DOTEnabled  bool   `json:"dot_enabled,omitempty"`
	DOTBind     string `json:"dot_bind,omitempty"`
	DOTPort     string `json:"dot_port,omitempty"`
	DOTCertFile string `json:"dot_cert_file,omitempty"`
	DOTKeyFile  string `json:"dot_key_file,omitempty"`

	// Inbound DoH (DNS over HTTPS) — requires TLS; separate from REST API port.
	DOHEnabled  bool   `json:"doh_enabled,omitempty"`
	DOHBind     string `json:"doh_bind,omitempty"`
	DOHPort     string `json:"doh_port,omitempty"`
	DOHPath     string `json:"doh_path,omitempty"`
	DOHCertFile string `json:"doh_cert_file,omitempty"`
	DOHKeyFile  string `json:"doh_key_file,omitempty"`

	// DNSResponseLimitMode is "sliding_window" (default) or "rrl" for extra response-side abuse control (0 = use sliding params only when mode set).
	DNSResponseLimitMode string `json:"dns_response_limit_mode,omitempty"`
	// DNSSlidingWindowSeconds is the window for sliding_window mode (default 1).
	DNSSlidingWindowSeconds int `json:"dns_sliding_window_seconds,omitempty"`
	// DNSMaxResponsesPerIPWindow caps responses per client IP per window (sliding_window).
	DNSMaxResponsesPerIPWindow int `json:"dns_max_responses_per_ip_window,omitempty"`
	// DNSRRLMaxPerBucket is max responses per (ip,qname) per rrl_window_seconds (rrl mode).
	DNSRRLMaxPerBucket  int     `json:"dns_rrl_max_per_bucket,omitempty"`
	DNSRRLWindowSeconds int     `json:"dns_rrl_window_seconds,omitempty"`
	DNSRRLSlip          float64 `json:"dns_rrl_slip,omitempty"`

	// DNSSECValidate enables best-effort RRSIG verification when DNSKEYs are present in upstream replies.
	DNSSECValidate        bool   `json:"dnssec_validate,omitempty"`
	DNSSECValidateStrict  bool   `json:"dnssec_validate_strict,omitempty"`
	DNSSECTrustAnchorFile string `json:"dnssec_trust_anchor_file,omitempty"` // reserved for future chain validation

	// DNSSECSignEnabled signs authoritative local answers (dnsrecords) with RRSIG when the client sets DO.
	DNSSECSignEnabled bool `json:"dnssec_sign_enabled,omitempty"`
	// DNSSECSignZone is the zone apex FQDN (e.g. "example.com.") for names covered by signing.
	DNSSECSignZone string `json:"dnssec_sign_zone,omitempty"`
	// DNSSECSignKeyFile is the path to the public DNSKEY file (BIND K*.key).
	DNSSECSignKeyFile string `json:"dnssec_sign_key_file,omitempty"`
	// DNSSECSignPrivateKeyFile is the path to the private key file (BIND K*.private).
	DNSSECSignPrivateKeyFile string `json:"dnssec_sign_private_key_file,omitempty"`

	// DNSRefuseANY when true returns NOTIMP for ANY queries.
	DNSRefuseANY bool `json:"dns_refuse_any,omitempty"`
	// DNSMaxEDNSUDPPayload caps the EDNS UDP payload size on responses (0 = no change). Suggested 1232.
	DNSMaxEDNSUDPPayload uint16            `json:"dns_max_edns_udp_payload,omitempty"`
	CacheRecords         bool              `json:"cache_records"`
	FullStats            bool              `json:"full_stats"`
	FullStatsDir         string            `json:"full_stats_dir"`
	ClientSocketPath     string            `json:"server_socket"`
	ClientTCPAddress     string            `json:"server_tcp"`
	FileLocations        FileLocations     `json:"file_locations"`
	DNSRecordSettings    DNSRecordSettings `json:"DNSRecordSettings"`
	Log                  LogConfig         `json:"log"`
	// AdblockListFiles is a list of paths to adblock list files (e.g. hosts-style). Loaded in order at startup and merged into a single block list.
	AdblockListFiles []string `json:"adblock_list_files,omitempty"`
	// UpstreamHealthCheckEnabled runs periodic probes and excludes failing upstreams from forwarding until they recover.
	UpstreamHealthCheckEnabled bool `json:"upstream_health_check_enabled,omitempty"`
	// UpstreamHealthCheckFailures is consecutive probe failures before marking an upstream unhealthy (default 3).
	UpstreamHealthCheckFailures int `json:"upstream_health_check_failures,omitempty"`
	// UpstreamHealthCheckIntervalSeconds is seconds between probe rounds (default 30).
	UpstreamHealthCheckIntervalSeconds int `json:"upstream_health_check_interval_seconds,omitempty"`
	// UpstreamHealthCheckQueryName is the QNAME for probes (default "google.com.").
	UpstreamHealthCheckQueryName string `json:"upstream_health_check_query_name,omitempty"`
	// MinCacheTTLSeconds overrides short upstream TTLs: cached answers use max(original TTL, this value).
	// Default 600 (10 minutes). Set to 0 to disable (use upstream TTL as-is).
	MinCacheTTLSeconds int `json:"min_cache_ttl_seconds,omitempty"`
	// StaleWhileRevalidate when true serves expired cache entries immediately (with TTL=1) while
	// refreshing from upstream in the background. Eliminates latency spikes on cache expiry.
	StaleWhileRevalidate bool `json:"stale_while_revalidate,omitempty"`
	// CacheWarmEnabled runs a background self-query to keep the Go process hot (CPU caches, memory pages).
	// Prevents cold-start latency spikes after idle periods. Default true.
	CacheWarmEnabled bool `json:"cache_warm_enabled,omitempty"`
	// CacheWarmIntervalSeconds is seconds between keep-alive self-queries. Default 10.
	CacheWarmIntervalSeconds int `json:"cache_warm_interval_seconds,omitempty"`
	// StatsPageEnabled serves GET /stats/page (HTML). Default true.
	StatsPageEnabled bool `json:"stats_page_enabled,omitempty"`
	// StatsPerfPageEnabled serves GET /stats/perf/page (HTML). Default true.
	StatsPerfPageEnabled bool `json:"stats_perf_page_enabled,omitempty"`
	// StatsDashboardEnabled serves GET /stats/dashboard and /stats/dashboard/data. Default true.
	StatsDashboardEnabled bool `json:"stats_dashboard_enabled,omitempty"`
	// PprofEnabled when true starts an HTTP server for Go runtime/pprof (CPU, heap, mutex, block, etc.).
	// Default false. Bind address is PprofListen (default 127.0.0.1:6060 when enabled).
	PprofEnabled bool `json:"pprof_enabled,omitempty"`
	// PprofListen is the listen address for the pprof HTTP server (e.g. "127.0.0.1:6060"). Empty uses default when PprofEnabled.
	PprofListen string `json:"pprof_listen,omitempty"`
	// PrettyJSON when true writes indented JSON for dnscache, dnsservers, and dnsrecords saves. Default false (compact; less CPU on large cache).
	PrettyJSON bool `json:"pretty_json,omitempty"`
	// ClusterEnabled turns on multi-node DNS record sync over TCP (see docs/clustering.md).
	ClusterEnabled bool `json:"cluster_enabled,omitempty"`
	// ClusterListenAddr is the TCP listen address for incoming cluster connections (e.g. ":7946"). Empty uses :7946 when enabled.
	ClusterListenAddr string `json:"cluster_listen_addr,omitempty"`
	// ClusterPeers lists peer host:port endpoints to push to after local record changes.
	ClusterPeers []string `json:"cluster_peers,omitempty"`
	// ClusterAuthToken is a shared secret; peers must match to exchange data.
	ClusterAuthToken string `json:"cluster_auth_token,omitempty"`
	// ClusterNodeID optionally identifies this node (stable across restarts). Empty: auto from cluster_state.json.
	ClusterNodeID string `json:"cluster_node_id,omitempty"`
	// ClusterSyncIntervalSeconds is periodic pull from peers (0 = disabled). Default 0.
	ClusterSyncIntervalSeconds int `json:"cluster_sync_interval_seconds,omitempty"`
	// ClusterAdvertiseAddr is the host:port peers should dial (shown in cluster info). Empty: derive from listen + guessed IP.
	ClusterAdvertiseAddr string `json:"cluster_advertise_addr,omitempty"`
	// ClusterReplicaOnly when true: pull/apply snapshots but do not push to peers (read replica).
	ClusterReplicaOnly bool `json:"cluster_replica_only,omitempty"`
	// ClusterRejectLocalWrites when true: reject local API/TUI record mutations (cluster applies still allowed).
	ClusterRejectLocalWrites bool `json:"cluster_reject_local_writes,omitempty"`
	// ClusterAdmin when true: this node may send admin_config_apply to peers (requires cluster_admin_token on both sides).
	ClusterAdmin bool `json:"cluster_admin,omitempty"`
	// ClusterAdminToken must match incoming admin_config_apply; empty disables remote admin apply.
	ClusterAdminToken string `json:"cluster_admin_token,omitempty"`
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
		MinCacheTTLSeconds:       600,
		StaleWhileRevalidate:     true,
		CacheWarmEnabled:         true,
		CacheWarmIntervalSeconds: 10,
		StatsPageEnabled:         true,
		StatsPerfPageEnabled:     true,
		StatsDashboardEnabled:    true,
		PrettyJSON:               false,
		PprofEnabled:             false,
		PprofListen:              "",
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
	if c.UpstreamHealthCheckFailures < 0 {
		c.UpstreamHealthCheckFailures = 0
	}
	if c.UpstreamHealthCheckIntervalSeconds < 0 {
		c.UpstreamHealthCheckIntervalSeconds = 0
	}
	if c.MinCacheTTLSeconds < 0 {
		c.MinCacheTTLSeconds = 0
	}
	if c.CacheWarmIntervalSeconds < 1 {
		c.CacheWarmIntervalSeconds = 10
	}
	if c.ClusterSyncIntervalSeconds < 0 {
		c.ClusterSyncIntervalSeconds = 0
	}
	if c.APIRateLimitBurst < 0 {
		c.APIRateLimitBurst = 0
	}
	if c.DNSRateLimitBurst < 0 {
		c.DNSRateLimitBurst = 0
	}
	if c.DNSAmplificationMaxRatio < 0 {
		c.DNSAmplificationMaxRatio = 0
	}
	if strings.TrimSpace(c.DNSResponseLimitMode) == "" {
		c.DNSResponseLimitMode = "sliding_window"
	}
	if c.DNSRRLWindowSeconds < 0 {
		c.DNSRRLWindowSeconds = 0
	}
	if c.DNSRRLSlip < 0 {
		c.DNSRRLSlip = 0
	}
	if c.DNSSlidingWindowSeconds < 0 {
		c.DNSSlidingWindowSeconds = 0
	}
	if c.PprofEnabled && strings.TrimSpace(c.PprofListen) == "" {
		c.PprofListen = "127.0.0.1:6060"
	}
	if c.DOTPort == "" && c.DOTEnabled {
		c.DOTPort = "853"
	}
	if c.DOHPort == "" && c.DOHEnabled {
		c.DOHPort = "8443"
	}
	if c.DOHPath == "" && c.DOHEnabled {
		c.DOHPath = "/dns-query"
	}

	if k := strings.TrimSpace(c.DNSSECSignKeyFile); k != "" {
		if filepath.IsAbs(k) {
			c.DNSSECSignKeyFile = filepath.Clean(k)
		} else {
			c.DNSSECSignKeyFile = filepath.Join(configDir, k)
		}
	}
	if k := strings.TrimSpace(c.DNSSECSignPrivateKeyFile); k != "" {
		if filepath.IsAbs(k) {
			c.DNSSECSignPrivateKeyFile = filepath.Clean(k)
		} else {
			c.DNSSECSignPrivateKeyFile = filepath.Join(configDir, k)
		}
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
	if r, ok := raw["api_auth_token"]; ok {
		_ = json.Unmarshal(r, &c.APIAuthToken)
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
	if r, ok := raw["adblock_list_files"]; ok {
		_ = json.Unmarshal(r, &c.AdblockListFiles)
	}
	if r, ok := raw["upstream_health_check_enabled"]; ok {
		_ = json.Unmarshal(r, &c.UpstreamHealthCheckEnabled)
	}
	if r, ok := raw["upstream_health_check_failures"]; ok {
		_ = json.Unmarshal(r, &c.UpstreamHealthCheckFailures)
	}
	if r, ok := raw["upstream_health_check_interval_seconds"]; ok {
		_ = json.Unmarshal(r, &c.UpstreamHealthCheckIntervalSeconds)
	}
	if r, ok := raw["upstream_health_check_query_name"]; ok {
		_ = json.Unmarshal(r, &c.UpstreamHealthCheckQueryName)
	}
	if r, ok := raw["min_cache_ttl_seconds"]; ok {
		_ = json.Unmarshal(r, &c.MinCacheTTLSeconds)
	}
	if r, ok := raw["stale_while_revalidate"]; ok {
		_ = json.Unmarshal(r, &c.StaleWhileRevalidate)
	}
	if r, ok := raw["cache_warm_enabled"]; ok {
		_ = json.Unmarshal(r, &c.CacheWarmEnabled)
	}
	if r, ok := raw["cache_warm_interval_seconds"]; ok {
		_ = json.Unmarshal(r, &c.CacheWarmIntervalSeconds)
	}
	if r, ok := raw["stats_page_enabled"]; ok {
		_ = json.Unmarshal(r, &c.StatsPageEnabled)
	}
	if r, ok := raw["stats_perf_page_enabled"]; ok {
		_ = json.Unmarshal(r, &c.StatsPerfPageEnabled)
	}
	if r, ok := raw["stats_dashboard_enabled"]; ok {
		_ = json.Unmarshal(r, &c.StatsDashboardEnabled)
	}
	if r, ok := raw["pprof_enabled"]; ok {
		_ = json.Unmarshal(r, &c.PprofEnabled)
	}
	if r, ok := raw["pprof_listen"]; ok {
		_ = json.Unmarshal(r, &c.PprofListen)
	}
	if r, ok := raw["pretty_json"]; ok {
		_ = json.Unmarshal(r, &c.PrettyJSON)
	}
	// Cache warm: default on when keys absent (legacy configs).
	if _, ok := raw["cache_warm_enabled"]; !ok {
		c.CacheWarmEnabled = true
	}
	if _, ok := raw["cache_warm_interval_seconds"]; !ok {
		c.CacheWarmIntervalSeconds = 10
	}
	// Opt-out HTML stats UIs: default true when keys are absent (legacy configs).
	if _, ok := raw["stats_page_enabled"]; !ok {
		c.StatsPageEnabled = true
	}
	if _, ok := raw["stats_perf_page_enabled"]; !ok {
		c.StatsPerfPageEnabled = true
	}
	if _, ok := raw["stats_dashboard_enabled"]; !ok {
		c.StatsDashboardEnabled = true
	}
	if r, ok := raw["cluster_enabled"]; ok {
		_ = json.Unmarshal(r, &c.ClusterEnabled)
	}
	if r, ok := raw["cluster_listen_addr"]; ok {
		_ = json.Unmarshal(r, &c.ClusterListenAddr)
	}
	if r, ok := raw["cluster_peers"]; ok {
		_ = json.Unmarshal(r, &c.ClusterPeers)
	}
	if r, ok := raw["cluster_auth_token"]; ok {
		_ = json.Unmarshal(r, &c.ClusterAuthToken)
	}
	if r, ok := raw["cluster_node_id"]; ok {
		_ = json.Unmarshal(r, &c.ClusterNodeID)
	}
	if r, ok := raw["cluster_sync_interval_seconds"]; ok {
		_ = json.Unmarshal(r, &c.ClusterSyncIntervalSeconds)
	}
	if r, ok := raw["cluster_advertise_addr"]; ok {
		_ = json.Unmarshal(r, &c.ClusterAdvertiseAddr)
	}
	if r, ok := raw["cluster_replica_only"]; ok {
		_ = json.Unmarshal(r, &c.ClusterReplicaOnly)
	}
	if r, ok := raw["cluster_reject_local_writes"]; ok {
		_ = json.Unmarshal(r, &c.ClusterRejectLocalWrites)
	}
	if r, ok := raw["cluster_admin"]; ok {
		_ = json.Unmarshal(r, &c.ClusterAdmin)
	}
	if r, ok := raw["cluster_admin_token"]; ok {
		_ = json.Unmarshal(r, &c.ClusterAdminToken)
	}
	return nil
}
