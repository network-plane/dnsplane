// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"dnsplane/api"
	"dnsplane/cluster"
	"dnsplane/commandhandler"
	"dnsplane/config"
	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnssecsign"
	"dnsplane/fullstats"
	"dnsplane/logger"
	"dnsplane/ratelimit"
	"dnsplane/resolver"

	"github.com/chzyer/readline"
	"github.com/inconshreveable/mousetrap"
	tui "github.com/network-plane/planetui"
	"github.com/spf13/cobra"
)

const (
	defaultTCPTerminalAddr = ":8053"
	defaultClientTCPPort   = "8053"
	// TCP TUI handshake prefix; client rejects connections without it.
	tuiBannerPrefix  = "dnsplane-tui"
	tuiBannerBusy    = "dnsplane-tui-busy"
	tuiClientKillCmd = "dnsplane-kill"
)

// Default release string for normal builds; RPM/DEB may override with -ldflags "-X main.appVersion=..." (see packaging/version.sh).
var appVersion = "1.4.185"

var (
	appState         = daemon.NewState()
	dnsResolver      *resolver.Resolver
	fullStatsTracker *fullstats.Tracker
	dnsLogger        *slog.Logger
	apiLogger        *slog.Logger
	tuiLogger        *slog.Logger
	clientLogger     *slog.Logger
	asyncLogQueue    *logger.AsyncLogQueue
	dnsQueryLimiter  *ratelimit.PerIP

	// defaultSocketPath is set in init() from config.DefaultClientSocketPath().
	defaultSocketPath string

	tcpTUIListenerMu sync.Mutex
	tcpTUIListener   net.Listener
	rootCmd          = &cobra.Command{
		Use:           "dnsplane",
		Short:         "DNS Server with optional CLI mode",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run the DNS server and optional TUI/API listeners",
		RunE:  runServer,
	}
	clientCmd = &cobra.Command{
		Use:   "client [socket-or-address]",
		Short: "Connect to a running dnsplane server (TUI client)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClient,
	}
)

// resolveDataPath returns the file path for a data file. If value is empty,
// returns cwd/defaultFileName. If value is a directory (ends with /, exists as
// dir, or base has no extension), returns value/defaultFileName; else returns value.
func resolveDataPath(value, defaultFileName, cwd string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filepath.Join(cwd, defaultFileName)
	}
	value = filepath.Clean(value)
	isDir := strings.HasSuffix(value, string(filepath.Separator))
	if !isDir {
		if info, err := os.Stat(value); err == nil && info.IsDir() {
			isDir = true
		} else if !strings.Contains(filepath.Base(value), ".") {
			isDir = true
		}
	}
	if isDir {
		value = strings.TrimSuffix(value, string(filepath.Separator))
		return filepath.Join(value, defaultFileName)
	}
	return value
}

func main() {
	if mousetrap.StartedByExplorer() {
		fmt.Fprintln(os.Stderr, "Please check the configuration in the repo.")
		os.Exit(1)
	}
	rootCmd.Version = fmt.Sprintf("DNS Resolver %s", appVersion)
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		slog.Default().Error("command failed", "error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serverCmd, clientCmd)
	defaultSocketPath = config.DefaultClientSocketPath()

	// Server flags
	serverCmd.Flags().String("config", "", "Path to config file (default: standard search order)")
	serverCmd.Flags().String("port", "53", "Port for DNS server")
	serverCmd.Flags().String("apiport", "8080", "Port for the REST API")
	serverCmd.Flags().Bool("api", false, "Enable the REST API")
	serverCmd.Flags().String("dnsservers", "", "Path to dnsservers.json file (overrides config)")
	serverCmd.Flags().String("dnsrecords", "", "Path to dnsrecords.json file (overrides config)")
	serverCmd.Flags().String("cache", "", "Path to dnscache.json file (overrides config)")
	serverCmd.Flags().String("server-socket", defaultSocketPath, "Path to UNIX domain socket for the daemon listener")
	serverCmd.Flags().String("server-tcp", defaultTCPTerminalAddr, "TCP address for remote TUI clients")

	// Client flags
	clientCmd.Flags().String("log-file", "", "Path to log file or directory for client (writes dnsplaneclient.log when set)")
	clientCmd.Flags().Bool("kill", false, "Disconnect the current TUI client and take over the session")

	api.SetAppVersion(appVersion)
}

func normalizeTCPAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	if !strings.Contains(addr, ":") {
		return "0.0.0.0:" + addr
	}
	return addr
}

func runServer(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	configPath, _ := cmd.Flags().GetString("config")

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	var loadedCfg *config.Loaded
	if configPath != "" {
		loadedCfg, err = config.LoadFromPath(configPath)
	} else {
		loadedCfg, err = config.Load()
	}
	if err != nil {
		return err
	}
	dnsservers, _ := cmd.Flags().GetString("dnsservers")
	dnsrecords, _ := cmd.Flags().GetString("dnsrecords")
	cache, _ := cmd.Flags().GetString("cache")
	loadedCfg.Config.FileLocations.DNSServerFile = resolveDataPath(dnsservers, "dnsservers.json", cwd)
	if dnsrecords != "" {
		loadedCfg.Config.FileLocations.RecordsSource = &config.RecordsSourceConfig{Type: config.RecordsSourceFile, Location: resolveDataPath(dnsrecords, "dnsrecords.json", cwd)}
	}
	loadedCfg.Config.FileLocations.CacheFile = resolveDataPath(cache, "dnscache.json", cwd)
	data.SetConfig(loadedCfg)

	logCfg := loadedCfg.Config.Log
	dnsLogger = logger.NewServerLogger(logger.DNSServerLog, logCfg.Dir, logCfg)
	apiLogger = logger.NewServerLogger(logger.APIServerLog, logCfg.Dir, logCfg)
	tuiLogger = logger.NewServerLogger(logger.TUIServerLog, logCfg.Dir, logCfg)
	asyncLogQueue = logger.NewAsyncLogQueue(0)
	data.SetResolverLogger(dnsLogger)

	if loadedCfg.Created {
		dnsLogger.Info("Created default config", "path", loadedCfg.Path)
	} else {
		dnsLogger.Debug("Config loaded", "path", loadedCfg.Path)
	}

	data.InitializeJSONFiles()
	dnsData := data.GetInstance()
	settings := dnsData.GetResolverSettings()

	port := settings.DNSPort
	if cmd.Flags().Changed("port") {
		if v, err := cmd.Flags().GetString("port"); err == nil {
			port = v
		}
	}

	serverSocket := settings.ClientSocketPath
	if cmd.Flags().Changed("server-socket") {
		if v, err := cmd.Flags().GetString("server-socket"); err == nil {
			serverSocket = v
		}
	}

	serverTCP := settings.ClientTCPAddress
	if cmd.Flags().Changed("server-tcp") {
		if v, err := cmd.Flags().GetString("server-tcp"); err == nil {
			serverTCP = v
		}
	}

	apiMode := settings.APIEnabled
	if cmd.Flags().Changed("api") {
		if v, err := cmd.Flags().GetBool("api"); err == nil {
			apiMode = v
		}
	}

	apiport := settings.RESTPort
	if cmd.Flags().Changed("apiport") {
		if v, err := cmd.Flags().GetString("apiport"); err == nil {
			apiport = v
		}
	}

	port = strings.TrimSpace(port)
	serverSocket = strings.TrimSpace(serverSocket)
	serverTCP = strings.TrimSpace(serverTCP)
	apiport = strings.TrimSpace(apiport)

	normalisedTCP := normalizeTCPAddress(serverTCP)
	appState.UpdateListener(func(info *daemon.ListenerSettings) {
		info.ClientSocketPath = serverSocket
		info.ClientTCPAddress = normalisedTCP
		info.DNSPort = port
		info.APIPort = apiport
		info.APIEndpoint = ""
		if info.APIPort != "" {
			if b := strings.TrimSpace(settings.APIBind); b != "" {
				info.APIEndpoint = normalizeTCPAddress(net.JoinHostPort(b, info.APIPort))
			} else {
				info.APIEndpoint = normalizeTCPAddress(":" + info.APIPort)
			}
		}
		info.APIEnabled = apiMode
	})
	if !apiMode {
		appState.SetAPIRunning(false)
	}

	settings.DNSPort = port
	settings.ClientSocketPath = serverSocket
	settings.ClientTCPAddress = normalisedTCP
	settings.APIEnabled = apiMode
	settings.RESTPort = apiport
	dnsData.UpdateSettingsInMemory(settings)

	if settings.DNSRateLimitPerIP > 0 {
		burst := settings.DNSRateLimitBurst
		if burst <= 0 {
			burst = 50
		}
		dnsQueryLimiter = ratelimit.NewPerIP(settings.DNSRateLimitPerIP, burst)
	} else {
		dnsQueryLimiter = nil
	}
	responseLimiter = buildResponseLimiter(settings)

	// Initialize full stats tracker if enabled
	if settings.FullStats {
		tracker, err := fullstats.New(settings.FullStatsDir, true)
		if err != nil {
			dnsLogger.Warn("Failed to initialize full stats tracker; fullstats disabled", "error", err)
		} else {
			fullStatsTracker = tracker
			dnsLogger.Info("Full statistics tracking enabled", "dir", settings.FullStatsDir)
		}
	}

	commandhandler.RegisterCommands()
	commandhandler.RegisterServerControlHooks(
		func() { stopDNSServer(appState) },
		func(p string) { restartDNSServer(appState, p) },
		func() bool { return getServerStatus(appState) },
		func(p string) { startAPIAsync(appState, p, apiLogger) },
		func() { stopAPIAsync(appState) },
		func() { startClientTCPListener(appState, tuiLogger) },
		func() { stopClientTCPListener(tuiLogger) },
		func() bool { return appState.IsTUIClientTCP() },
		func() commandhandler.ServerListenerInfo { return currentServerListeners(appState) },
	)
	commandhandler.SetVersion(appVersion, appVersion)
	commandhandler.SetFullStatsTracker(fullStatsTracker)
	api.SetFullStatsTracker(fullStatsTracker)
	tui.SetPrompt("dnsplane> ")

	appState.SetDaemonMode(true)

	if apiMode {
		startAPIAsync(appState, apiport, apiLogger)
	}

	if dnsResolver == nil {
		var dnssecZSK *dnssecsign.Signer
		signSt := dnsData.GetResolverSettings()
		if signSt.DNSSECSignEnabled && strings.TrimSpace(signSt.DNSSECSignZone) != "" &&
			strings.TrimSpace(signSt.DNSSECSignKeyFile) != "" && strings.TrimSpace(signSt.DNSSECSignPrivateKeyFile) != "" {
			if s, err := dnssecsign.LoadSigner(signSt.DNSSECSignZone, signSt.DNSSECSignKeyFile, signSt.DNSSECSignPrivateKeyFile); err != nil {
				if dnsLogger != nil {
					dnsLogger.Warn("DNSSEC signing disabled: key load failed", "error", err)
				}
			} else {
				dnssecZSK = s
				if dnsLogger != nil {
					dnsLogger.Info("DNSSEC signing enabled for zone", "zone", dnssecZSK.Zone())
				}
			}
		}
		dnsResolver = resolver.New(resolver.Config{
			Store:        dnsData,
			Upstream:     resolver.NewDNSClient(2 * time.Second),
			DNSSECSigner: dnssecZSK,
			Logger: func(format string, args ...interface{}) {
				if asyncLogQueue != nil {
					asyncLogQueue.Enqueue(func() {
						if appState.DaemonMode() && dnsLogger != nil {
							dnsLogger.Debug(fmt.Sprintf(strings.TrimSuffix(format, "\n"), args...))
						}
					})
				}
			},
			ErrorLogger: func(msg string, keyValues ...any) {
				if asyncLogQueue != nil && dnsLogger != nil {
					asyncLogQueue.Enqueue(func() { dnsLogger.Error(msg, keyValues...) })
				}
			},
			QueryObserver: func(qname, qtype, outcome, upstream, recordSummary string, elapsed time.Duration, clientIP string) {
				ms := elapsed.Seconds() * 1000
				cip := strings.TrimSpace(clientIP)
				if cip == "" {
					cip = "unknown"
				}
				data.RecordDashboardResolution(data.DashboardResolution{
					ClientIP:   cip,
					Qname:      qname,
					Qtype:      qtype,
					Outcome:    outcome,
					Upstream:   upstream,
					Record:     recordSummary,
					DurationMs: ms,
				})
				if fullStatsTracker != nil {
					key := fmt.Sprintf("%s:%s", qname, qtype)
					_ = fullStatsTracker.RecordRequest(key, cip, qtype, outcome)
				}
				if asyncLogQueue == nil || dnsLogger == nil || !appState.DaemonMode() {
					return
				}
				asyncLogQueue.Enqueue(func() {
					dnsLogger.Debug("dns query",
						"qname", qname,
						"qtype", qtype,
						"outcome", outcome,
						"upstream", upstream,
						"record", recordSummary,
						"duration_ms", ms,
					)
				})
			},
			UpstreamTimeout: 2 * time.Second,
		})
	}

	startedCh, dnsErrCh := startDNSServer(appState, port)

	waitForServer := func() error {
		select {
		case <-startedCh:
			return nil
		case err := <-dnsErrCh:
			return err
		}
	}

	if err := waitForServer(); err != nil {
		dnsLogger.Error("Error starting DNS server", "error", err)
		fmt.Fprintf(os.Stderr, "Error starting DNS server: %v\n", err)
		return nil
	}

	monitorDNSErrors := func() {
		go func() {
			if err := <-dnsErrCh; err != nil {
				dnsLogger.Error("DNS server error", "error", err)
				fmt.Fprintf(os.Stderr, "DNS server error: %v\n", err)
			}
		}()
	}

	monitorDNSErrors()

	go startInboundDNSListeners(appState.StopChannel())

	go runUpstreamHealthProbeLoop(dnsData, dnsLogger)
	go runCacheWarmLoop(dnsData, port)
	go runCacheCompactLoop(dnsData, dnsLogger)

	if s := dnsData.GetResolverSettings(); s.PprofEnabled {
		addr := strings.TrimSpace(s.PprofListen)
		if addr != "" {
			startPprof(addr, dnsLogger)
		}
	}

	appState.SetReadlineConfig(readline.Config{
		Prompt:                 "> ",
		HistoryFile:            "/tmp/dnsplane.history",
		DisableAutoSaveHistory: false, // Enable auto-save so history persists
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		HistorySearchFold:      true,
		FuncGetWidth:           func() int { return 80 }, // Fix: prevent -1 width causing cursor-up on each key
	})

	var unixListener net.Listener
	var tcpListener net.Listener
	if serverSocket != "" {
		listener, err := startUnixSocketListener(serverSocket, tuiLogger)
		if err != nil {
			if tuiLogger != nil {
				tuiLogger.Error("failed to start UNIX socket listener", "path", serverSocket, "error", err)
			}
			return fmt.Errorf("unix socket listener error: %w", err)
		}
		unixListener = listener
		go acceptInteractiveSessions(listener, tuiLogger)
	}
	if serverTCP != "" {
		listener, err := startTCPTerminalListener(serverTCP, tuiLogger)
		if err != nil {
			if tuiLogger != nil {
				tuiLogger.Error("failed to start TCP TUI listener", "address", serverTCP, "error", err)
			}
			return fmt.Errorf("tcp listener error: %w", err)
		}
		tcpListener = listener
		tcpTUIListenerMu.Lock()
		tcpTUIListener = listener
		tcpTUIListenerMu.Unlock()
		go acceptInteractiveSessions(listener, tuiLogger)
	}

	clusterCtx, clusterCancel := context.WithCancel(context.Background())
	clusterMgr := cluster.NewManager(loadedCfg.Path, dnsData, dnsLogger)
	cluster.SetGlobalManager(clusterMgr)
	sCluster := dnsData.GetResolverSettings()
	if err := clusterMgr.Start(clusterCtx, sCluster); err != nil && dnsLogger != nil {
		dnsLogger.Warn("cluster: start failed", "error", err)
	}
	if sCluster.ClusterEnabled {
		data.SetClusterRecordsNotify(clusterMgr.NotifyLocalRecordsChanged)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	if dnsLogger != nil {
		dnsLogger.Info("Server running; press Ctrl+C to exit")
	}
	fmt.Println("Press Ctrl+C to exit daemon mode.")
	<-sigCh
	if dnsLogger != nil {
		dnsLogger.Info("Shutting down")
	}
	fmt.Println("Shutting down.")
	stopPprof()
	clusterCancel()
	clusterMgr.Stop()
	cluster.SetGlobalManager(nil)
	data.SetClusterRecordsNotify(nil)
	stopDNSServer(appState)
	if fullStatsTracker != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			if err := fullStatsTracker.Close(); err != nil && dnsLogger != nil {
				dnsLogger.Warn("Error closing full stats tracker on shutdown", "error", err)
			}
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			if dnsLogger != nil {
				dnsLogger.Warn("Full stats tracker close timed out; proceeding with shutdown")
			}
		}
	}
	if asyncLogQueue != nil {
		asyncLogQueue.Close()
	}
	if dnsData := data.GetInstance(); dnsData != nil {
		dnsData.Close()
	}
	if unixListener != nil {
		_ = unixListener.Close()
		if tuiLogger != nil {
			tuiLogger.Debug("UNIX socket listener closed")
		}
	}
	if tcpListener != nil {
		_ = tcpListener.Close()
		tcpTUIListenerMu.Lock()
		tcpTUIListener = nil
		tcpTUIListenerMu.Unlock()
		if tuiLogger != nil {
			tuiLogger.Debug("TCP TUI listener closed")
		}
	}
	if serverSocket != "" {
		_ = syscall.Unlink(serverSocket)
	}
	return nil
}
