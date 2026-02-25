package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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
	"dnsplane/commandhandler"
	"dnsplane/config"
	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/fullstats"
	"dnsplane/logger"
	"dnsplane/resolver"

	"github.com/chzyer/readline"
	"github.com/inconshreveable/mousetrap"
	"github.com/miekg/dns"
	tui "github.com/network-plane/planetui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	defaultTCPTerminalAddr = ":8053"
	defaultClientTCPPort   = "8053"
	// tuiBannerPrefix is sent by the server on new TUI connections; client must see this or disconnect.
	tuiBannerPrefix  = "dnsplane-tui"
	tuiBannerBusy    = "dnsplane-tui-busy"
	tuiClientKillCmd = "dnsplane-kill"
)

var (
	appState         = daemon.NewState()
	appversion       = "1.3.87"
	dnsResolver      *resolver.Resolver
	fullStatsTracker *fullstats.Tracker
	dnsLogger        *slog.Logger
	apiLogger        *slog.Logger
	tuiLogger        *slog.Logger
	clientLogger     *slog.Logger
	asyncLogQueue    *logger.AsyncLogQueue

	// defaultSocketPath is the default UNIX socket for server and client (set in init from config).
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
		Use:   "client",
		Short: "Connect to a running dnsplane server (TUI client)",
		RunE:  runClient,
	}
)

func resetTUIState() {
	if mgr := tui.DefaultEngine().Contexts(); mgr != nil {
		_ = mgr.PopToRoot()
	}
}

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
	rootCmd.Version = fmt.Sprintf("DNS Resolver %s", appversion)
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
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
	clientCmd.Flags().String("client", "", "Socket path or address to connect to (default: "+defaultSocketPath+")")
	clientCmd.Flags().String("log-file", "", "Path to log file or directory for client (writes dnsplaneclient.log when set)")
	clientCmd.Flags().Bool("kill", false, "Disconnect the current TUI client and take over the session")
	if f := clientCmd.Flags().Lookup("client"); f != nil {
		f.NoOptDefVal = defaultSocketPath
	}
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

func runClient(cmd *cobra.Command, args []string) error {
	clientTarget, _ := cmd.Flags().GetString("client")
	if clientTarget == "" {
		clientTarget = defaultSocketPath
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		clientTarget = args[0]
		args = args[1:]
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	logFilePath, _ := cmd.Flags().GetString("log-file")
	if logFilePath != "" {
		clientLogger = logger.NewClientLogger(logFilePath)
		clientLogger.Info("client starting", "target", clientTarget)
	}
	killOther, _ := cmd.Flags().GetBool("kill")
	connectToInteractiveEndpoint(clientTarget, killOther)
	if clientLogger != nil {
		clientLogger.Debug("client exiting")
	}
	return nil
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

	logCfg := settings.Log
	dnsLogger = logger.NewServerLogger(logger.DNSServerLog, logCfg.Dir, logCfg)
	apiLogger = logger.NewServerLogger(logger.APIServerLog, logCfg.Dir, logCfg)
	tuiLogger = logger.NewServerLogger(logger.TUIServerLog, logCfg.Dir, logCfg)
	asyncLogQueue = logger.NewAsyncLogQueue(0)

	if loadedCfg.Created {
		dnsLogger.Info("Created default config", "path", loadedCfg.Path)
	} else {
		dnsLogger.Debug("Config loaded", "path", loadedCfg.Path)
	}

	normalisedTCP := normalizeTCPAddress(serverTCP)
	appState.UpdateListener(func(info *daemon.ListenerSettings) {
		info.ClientSocketPath = serverSocket
		info.ClientTCPAddress = normalisedTCP
		info.DNSPort = port
		info.APIPort = apiport
		info.APIEndpoint = ""
		if info.APIPort != "" {
			info.APIEndpoint = normalizeTCPAddress(":" + info.APIPort)
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
		isClientTCPListenerRunning,
		func() bool { return appState.IsTUIClientTCP() },
		func() commandhandler.ServerListenerInfo { return currentServerListeners(appState) },
	)
	commandhandler.SetVersion(appversion, appversion)
	commandhandler.SetFullStatsTracker(fullStatsTracker)
	api.SetFullStatsTracker(fullStatsTracker)
	tui.SetPrompt("dnsplane> ")

	appState.SetDaemonMode(true)

	if apiMode {
		startAPIAsync(appState, apiport, apiLogger)
	}

	if dnsResolver == nil {
		dnsResolver = resolver.New(resolver.Config{
			Store:    dnsData,
			Upstream: resolver.NewDNSClient(2 * time.Second),
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
			UpstreamTimeout: 2 * time.Second,
		})
	}

	dns.HandleFunc(".", handleRequest)

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

func stopAPIAsync(state *daemon.State) {
	api.Stop(state)
}

func startClientTCPListener(state *daemon.State, log *slog.Logger) {
	tcpTUIListenerMu.Lock()
	if tcpTUIListener != nil {
		tcpTUIListenerMu.Unlock()
		if log != nil {
			log.Info("TCP TUI listener already running")
		}
		fmt.Println("Client TCP listener already running.")
		return
	}
	addr := strings.TrimSpace(state.ListenerSnapshot().ClientTCPAddress)
	if addr == "" {
		addr = defaultTCPTerminalAddr
	}
	tcpTUIListenerMu.Unlock()
	listener, err := startTCPTerminalListener(addr, log)
	if err != nil {
		if log != nil {
			log.Error("failed to start TCP TUI listener", "address", addr, "error", err)
		}
		fmt.Printf("Failed to start TCP TUI listener: %v\n", err)
		return
	}
	tcpTUIListenerMu.Lock()
	tcpTUIListener = listener
	tcpTUIListenerMu.Unlock()
	go acceptInteractiveSessions(listener, log)
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.ClientTCPAddress = addr
	})
	if log != nil {
		log.Info("TCP TUI listener started", "address", addr)
	}
	fmt.Println("Client TCP listener started.")
}

func stopClientTCPListener(log *slog.Logger) {
	tcpTUIListenerMu.Lock()
	l := tcpTUIListener
	tcpTUIListener = nil
	tcpTUIListenerMu.Unlock()
	if l == nil {
		fmt.Println("Client TCP listener was not running.")
		return
	}
	_ = l.Close()
	if log != nil {
		log.Info("TCP TUI listener stopped")
	}
	fmt.Println("Client TCP listener stopped.")
}

func isClientTCPListenerRunning() bool {
	tcpTUIListenerMu.Lock()
	running := tcpTUIListener != nil
	tcpTUIListenerMu.Unlock()
	return running
}

func currentServerListeners(state *daemon.State) commandhandler.ServerListenerInfo {
	listener := state.ListenerSnapshot()
	dnsPort := strings.TrimSpace(listener.DNSPort)
	settings := data.GetInstance().GetResolverSettings()
	if dnsPort == "" {
		dnsPort = strings.TrimSpace(settings.DNSPort)
	}

	socket := strings.TrimSpace(listener.ClientSocketPath)
	tcp := strings.TrimSpace(listener.ClientTCPAddress)
	apiEndpoint := strings.TrimSpace(listener.APIEndpoint)
	if apiEndpoint == "" && settings.RESTPort != "" {
		apiEndpoint = normalizeTCPAddress(":" + strings.TrimSpace(settings.RESTPort))
	}

	info := commandhandler.ServerListenerInfo{
		DNSProtocol:         "udp",
		DNSListeners:        []string{normalizeTCPAddress(":" + dnsPort)},
		ClientSocket:        socket,
		ClientSocketEnabled: socket != "",
		ClientTCPEndpoint:   tcp,
		ClientTCPEnabled:    tcp != "",
		ClientTCPRunning:    isClientTCPListenerRunning(),
		APIEndpoint:         apiEndpoint,
		APIEnabled:          listener.APIEnabled,
		APIRunning:          state.APIRunning(),
	}
	return info
}

func startAPIAsync(state *daemon.State, port string, log *slog.Logger) {
	if port == "" {
		port = data.GetInstance().GetResolverSettings().RESTPort
	}
	trimmed := strings.TrimSpace(port)
	apiEndpoint := normalizeTCPAddress(":" + trimmed)
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.APIPort = trimmed
		info.APIEndpoint = apiEndpoint
		info.APIEnabled = true
	})
	if state.APIRunning() {
		return
	}
	api.Start(state, trimmed, nil, log)
}

func startDNSServer(state *daemon.State, port string) (<-chan struct{}, <-chan error) {
	trimmedPort := strings.TrimSpace(port)
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.DNSPort = trimmedPort
	})
	dnsData := data.GetInstance()

	server := &dns.Server{
		Addr:         fmt.Sprintf(":%s", trimmedPort),
		Net:          "udp",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	dnsLogger.Info("Starting DNS server", "addr", server.Addr)

	startedCh := make(chan struct{})
	errCh := make(chan error, 1)
	var once sync.Once

	server.NotifyStartedFunc = func() {
		once.Do(func() {
			state.SetServerStatus(true)
			stats := dnsData.GetStats()
			stats.ServerStartTime = time.Now()
			dnsData.UpdateStats(stats)
			close(startedCh)
		})
	}

	go func() {
		defer state.NotifyStopped()
		if err := server.ListenAndServe(); err != nil {
			state.SetServerStatus(false)
			select {
			case errCh <- err:
			default:
			}
			return
		}
		state.SetServerStatus(false)
	}()

	stopCh := state.StopChannel()
	go func() {
		<-stopCh
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.ShutdownContext(shutdownCtx); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	return startedCh, errCh
}

func restartDNSServer(state *daemon.State, port string) {
	if state.ServerStatus() {
		stopDNSServer(state)
	}
	state.ResetDNSChannels()
	startedCh, errCh := startDNSServer(state, port)
	select {
	case <-startedCh:
	case err := <-errCh:
		dnsLogger.Error("Error restarting DNS server", "error", err)
		fmt.Fprintf(os.Stderr, "Error restarting DNS server: %v\n", err)
	}
}

const dnsShutdownWaitTimeout = 20 * time.Second

func stopDNSServer(state *daemon.State) {
	if !state.ServerStatus() {
		return
	}
	stoppedCh := state.SignalStop()
	select {
	case <-stoppedCh:
		// normal shutdown
	case <-time.After(dnsShutdownWaitTimeout):
		if dnsLogger != nil {
			dnsLogger.Warn("DNS server shutdown timed out; proceeding anyway")
		}
	}
	state.SetServerStatus(false)
	if dnsLogger != nil {
		dnsLogger.Debug("DNS server stopped")
	}
}

func getServerStatus(state *daemon.State) bool {
	return state.ServerStatus()
}

// DNS
func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = false

	requesterIP := "unknown"
	if addr := writer.RemoteAddr(); addr != nil {
		if host, _, err := net.SplitHostPort(addr.String()); err == nil {
			requesterIP = host
		} else {
			requesterIP = addr.String()
		}
	}

	ctx := context.Background()
	if dnsResolver != nil {
		for _, question := range request.Question {
			dnsResolver.HandleQuestion(ctx, question, response)
		}
	}

	err := writer.WriteMsg(response)

	// Everything after the reply is async so the client never waits on logging or stats.
	go func() {
		dnsData := data.GetInstance()
		dnsData.IncrementTotalQueries()
		for _, question := range request.Question {
			if fullStatsTracker != nil {
				recordType := dns.TypeToString[question.Qtype]
				key := fmt.Sprintf("%s:%s", question.Name, recordType)
				_ = fullStatsTracker.RecordRequest(key, requesterIP, recordType)
			}
		}
		if err != nil && asyncLogQueue != nil && dnsLogger != nil {
			errCopy := err
			asyncLogQueue.Enqueue(func() { dnsLogger.Error("Error writing response", "error", errCopy) })
		}
	}()
}

func startUnixSocketListener(socketPath string, log *slog.Logger) (net.Listener, error) {
	if socketPath == "" {
		return nil, nil
	}
	if dir := filepath.Dir(socketPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create socket directory: %w", err)
		}
	}
	if err := syscall.Unlink(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove unix socket: %w", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.Info("Listening on UNIX socket", "path", socketPath)
	}
	return listener, nil
}

func startTCPTerminalListener(address string, log *slog.Logger) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.Info("Listening on TCP address for TUI clients", "address", address)
	}
	return listener, nil
}

func acceptInteractiveSessions(listener net.Listener, log *slog.Logger) {
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if log != nil {
					log.Warn("Temporary accept error", "error", err)
				}
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if log != nil {
				log.Error("Error accepting connection", "error", err)
			}
			return
		}
		go serveInteractiveSession(conn, log)
	}
}

func serveInteractiveSession(conn net.Conn, log *slog.Logger) {
	defer conn.Close()

	addr := formatConnAddr(conn)
	if log != nil {
		log.Info("TUI client connected", "addr", addr)
		defer func() { log.Debug("TUI client disconnected", "addr", addr) }()
	}

	// Read one optional line from client (e.g. dnsplane-kill to take over). Short timeout.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	clientLine, _ := bufio.NewReader(conn).ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	clientLine = strings.TrimSpace(clientLine)

	tuiLock := appState.TUISessionMutex()
	if strings.TrimSpace(clientLine) == tuiClientKillCmd {
		appState.DisconnectCurrentTUIClient()
		// Wait for the disconnected session to release the lock.
		tuiLock.Lock()
	} else {
		if !tuiLock.TryLock() {
			curAddr, curSince := appState.GetTUIClientInfo()
			sinceStr := ""
			if !curSince.IsZero() {
				sinceStr = curSince.Format(time.RFC3339)
			}
			if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err == nil {
				_, _ = fmt.Fprintf(conn, "%s %s %s\n", tuiBannerBusy, curAddr, sinceStr)
				_ = conn.SetWriteDeadline(time.Time{})
			}
			return
		}
	}
	defer tuiLock.Unlock()

	appState.SetTUIClientSession(conn, addr)
	defer appState.ClearTUIClientSession()

	// Send banner so remote client can verify this is a dnsplane server.
	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err == nil {
		_, _ = fmt.Fprintf(conn, "%s %s\n", tuiBannerPrefix, appversion)
		_ = conn.SetWriteDeadline(time.Time{})
	}

	// Wrap conn with CRLF writer so TUI output \n becomes \r\n
	crlfConn := &crlfWriter{w: conn}
	prevOutputWriter := tui.SetOutputWriter(crlfConn)
	defer tui.SetOutputWriter(prevOutputWriter)

	cfg := appState.ReadlineConfig()
	cfg.Stdin = conn
	cfg.Stdout = conn
	cfg.Stderr = conn
	if cfg.HistoryFile == "" {
		cfg.HistoryFile = "/tmp/dnsplane.history"
	}
	cfg.DisableAutoSaveHistory = false
	cfg.FuncMakeRaw = func() error { return nil }
	cfg.FuncExitRaw = func() error { return nil }
	cfg.FuncIsTerminal = func() bool { return true }
	cfg.FuncGetWidth = func() int { return 80 } // Fix: prevent -1 width causing cursor-up on each key
	cfg.ForceUseInteractive = true

	rl, err := readline.NewEx(&cfg)
	if err != nil {
		fmt.Fprintf(conn, "Error initialising session: %v\r\n", err)
		return
	}
	defer rl.Close()
	defer resetTUIState()
	resetTUIState()

	if err := tui.Run(rl); err != nil {
		fmt.Fprintf(conn, "\r\nSession terminated: %v\r\n", err)
	} else {
		fmt.Fprint(conn, "\rShutting down session.\r\n")
	}
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}

func formatConnAddr(conn net.Conn) string {
	if conn == nil {
		return "unknown"
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return "unknown"
	}
	s := addr.String()
	if addr.Network() == "unix" {
		if s == "" || s == "@" {
			return "unix-local"
		}
	}
	return s
}

type crlfWriter struct {
	w    io.Writer
	last byte
}

func (cw *crlfWriter) Write(p []byte) (int, error) {
	if cw == nil || cw.w == nil {
		return len(p), nil
	}
	buf := make([]byte, 0, len(p)+bytes.Count(p, []byte{'\n'}))
	prev := cw.last
	for _, b := range p {
		if b == '\n' {
			if prev == '\r' {
				buf = append(buf, '\n')
			} else {
				buf = append(buf, '\r', '\n')
			}
		} else {
			buf = append(buf, b)
		}
		prev = b
	}
	_, err := cw.w.Write(buf)
	if err != nil {
		return 0, err
	}
	cw.last = prev
	return len(p), nil
}

func connectToInteractiveEndpoint(target string, killOther bool) {
	network, address := resolveInteractiveTarget(target)
	conn, err := net.Dial(network, address)
	if err != nil {
		if clientLogger != nil {
			clientLogger.Error("connection failed", "target", address, "error", err)
		}
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %v\n", address, err)
		return
	}
	defer conn.Close()

	// If --kill, tell server to disconnect the current client so we can take over.
	if killOther {
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		_, _ = fmt.Fprintf(conn, "%s\n", tuiClientKillCmd)
		_ = conn.SetWriteDeadline(time.Time{})
	}

	// Verify server is dnsplane by reading the banner (avoids garbage/stuck when connecting to wrong service).
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	banner, err := reader.ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		if clientLogger != nil {
			clientLogger.Error("failed to read server banner", "address", address, "error", err)
		}
		fmt.Fprintf(os.Stderr, "Error: not a dnsplane server at %s (could not read banner: %v)\n", address, err)
		return
	}
	banner = strings.TrimSpace(banner)
	if strings.HasPrefix(banner, tuiBannerBusy) {
		// Another client is connected; server sent: dnsplane-tui-busy <addr> <since>
		rest := strings.TrimSpace(strings.TrimPrefix(banner, tuiBannerBusy))
		parts := strings.SplitN(rest, " ", 2)
		curAddr := "unknown"
		sinceStr := ""
		if len(parts) >= 1 && parts[0] != "" {
			curAddr = parts[0]
		}
		if len(parts) >= 2 {
			sinceStr = parts[1]
		}
		msg := fmt.Sprintf("Another client is already connected: %s", curAddr)
		if sinceStr != "" {
			msg += fmt.Sprintf(", connected since %s", sinceStr)
		}
		msg += ".\nUse --kill to disconnect that client and take over."
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		return
	}
	if !strings.HasPrefix(banner, tuiBannerPrefix) {
		if clientLogger != nil {
			clientLogger.Error("invalid server banner", "address", address, "banner", banner)
		}
		fmt.Fprintf(os.Stderr, "Error: not a dnsplane server at %s (got %q)\n", address, banner)
		return
	}

	serverVersion := strings.TrimSpace(strings.TrimPrefix(banner, tuiBannerPrefix))
	if serverVersion != "" && serverVersion != appversion {
		fmt.Fprintf(os.Stderr, "Warning: version mismatch — server %s, client %s\n", serverVersion, appversion)
		if clientLogger != nil {
			clientLogger.Warn("version mismatch", "server", serverVersion, "client", appversion)
		}
	}

	if clientLogger != nil {
		clientLogger.Info("connected", "network", network, "address", address)
	}
	fmt.Printf("Connected to %s %s\n", network, address)

	stdinFD := int(os.Stdin.Fd())
	var (
		oldState *term.State
		restored bool
	)
	if term.IsTerminal(stdinFD) {
		if st, err := term.MakeRaw(stdinFD); err == nil {
			oldState = st
			defer func() {
				if !restored {
					term.Restore(stdinFD, oldState)
				}
			}()
		}
	}

	var sigCh chan os.Signal
	if oldState != nil {
		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer func() {
			signal.Stop(sigCh)
			close(sigCh)
		}()
		go func() {
			<-sigCh
			restored = true
			_ = term.Restore(stdinFD, oldState)
			os.Exit(0)
		}()
	}

	readDone := make(chan struct{})
	writeDone := make(chan struct{})

	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		if cw, ok := conn.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = conn.Close()
		}
		close(writeDone)
	}()

	go func() {
		_, _ = io.Copy(os.Stdout, reader)
		close(readDone)
	}()

	readClosed := false
	writeClosed := false
	select {
	case <-readDone:
		readClosed = true
		_ = conn.Close()
	case <-writeDone:
		writeClosed = true
	}
	if !readClosed {
		<-readDone
	}
	if !writeClosed {
		_ = conn.Close()
	}

	if oldState != nil && !restored {
		restored = true
		_ = term.Restore(stdinFD, oldState)
	}

	if clientLogger != nil {
		clientLogger.Debug("connection closed", "address", address)
	}
	fmt.Println("Connection closed.")
}

func resolveInteractiveTarget(target string) (network, address string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "unix", defaultSocketPath
	}
	if strings.ContainsAny(target, "/\\") || strings.HasPrefix(target, "@") {
		return "unix", target
	}
	target = strings.TrimPrefix(target, "tcp://")
	if strings.HasPrefix(target, "unix://") {
		return "unix", strings.TrimPrefix(target, "unix://")
	}
	if host, port, err := net.SplitHostPort(target); err == nil {
		if host == "" {
			host = "127.0.0.1"
		}
		return "tcp", net.JoinHostPort(host, port)
	}
	host := target
	if host == "" {
		host = "127.0.0.1"
	}
	return "tcp", net.JoinHostPort(host, defaultClientTCPPort)
}
