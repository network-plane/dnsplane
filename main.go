package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/miekg/dns"
	tui "github.com/network-plane/planetui"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"dnsplane/api"
	"dnsplane/commandhandler"
	"dnsplane/config"
	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/fullstats"
	"dnsplane/resolver"
)

const (
	defaultUnixSocketPath  = "/tmp/dnsplane.socket"
	defaultTCPTerminalAddr = ":8053"
	defaultClientTCPPort   = "8053"
)

var (
	appState         = daemon.NewState()
	appversion       = "0.1.17"
	dnsResolver      *resolver.Resolver
	fullStatsTracker *fullstats.Tracker
	rootCmd          = &cobra.Command{
		Use:           "dnsplane",
		Short:         "DNS Server with optional CLI mode",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runRoot,
	}
)

func resetTUIState() {
	if mgr := tui.DefaultEngine().Contexts(); mgr != nil {
		_ = mgr.PopToRoot()
	}
}

func main() {
	loadedCfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	data.SetConfig(loadedCfg)
	if loadedCfg.Created {
		fmt.Printf("Created default config at %s\n", loadedCfg.Path)
	}

	// Create JSON files if they don't exist using configured locations.
	data.InitializeJSONFiles()

	// Ensure resolver settings are initialised before running commands
	data.GetInstance().GetResolverSettings()

	rootCmd.Version = fmt.Sprintf("DNS Resolver %s", appversion)
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.String("port", "53", "Port for DNS server")
	flags.String("server-socket", defaultUnixSocketPath, "Path to UNIX domain socket for the daemon listener")
	flags.String("server-tcp", defaultTCPTerminalAddr, "TCP address for remote TUI clients")
	flags.Bool("api", false, "Enable the REST API")
	flags.String("apiport", "8080", "Port for the REST API")
	flags.StringP("client", "", "", "Run in client mode socket or address (default: "+defaultUnixSocketPath+")")
	if f := flags.Lookup("client"); f != nil {
		f.NoOptDefVal = defaultUnixSocketPath
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

func runRoot(cmd *cobra.Command, args []string) error {
	remaining := append([]string(nil), args...)

	dnsData := data.GetInstance()
	settings := dnsData.GetResolverSettings()

	clientRequested := cmd.Flags().Changed("client")
	clientTarget, _ := cmd.Flags().GetString("client")
	if clientRequested {
		// If --server-tcp is set in client mode, use it as the connection target
		// This takes precedence over the default value from NoOptDefVal
		if cmd.Flags().Changed("server-tcp") {
			if serverTCP, err := cmd.Flags().GetString("server-tcp"); err == nil && serverTCP != "" {
				clientTarget = strings.TrimSpace(serverTCP)
			}
		}
		if clientTarget == "" {
			clientTarget = strings.TrimSpace(settings.ClientSocketPath)
			if clientTarget == "" {
				clientTarget = defaultUnixSocketPath
			}
		}
		if len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
			clientTarget = remaining[0]
			remaining = remaining[1:]
		}
		if len(remaining) > 0 {
			return fmt.Errorf("unexpected arguments: %v", remaining)
		}
		connectToInteractiveEndpoint(clientTarget)
		return nil
	}

	if len(remaining) > 0 {
		return fmt.Errorf("unexpected arguments: %v", remaining)
	}

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
			log.Printf("Warning: Failed to initialize full stats tracker: %v", err)
		} else {
			fullStatsTracker = tracker
			log.Printf("Full statistics tracking enabled, storing data in: %s", settings.FullStatsDir)
		}
	}

	commandhandler.RegisterCommands()
	commandhandler.RegisterServerControlHooks(
		func() { stopDNSServer(appState) },
		func(p string) { restartDNSServer(appState, p) },
		func() bool { return getServerStatus(appState) },
		func(p string) { startAPIAsync(appState, p) },
		func() commandhandler.ServerListenerInfo { return currentServerListeners(appState) },
	)
	tui.SetPrompt("dnsplane> ")

	appState.SetDaemonMode(true)

	if apiMode {
		startAPIAsync(appState, apiport)
	}

	if dnsResolver == nil {
		dnsResolver = resolver.New(resolver.Config{
			Store:           dnsData,
			Upstream:        resolver.NewDNSClient(2 * time.Second),
			Logger:          logQuery,
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
		fmt.Fprintf(os.Stderr, "Error starting DNS server: %v\n", err)
		return nil
	}

	monitorDNSErrors := func() {
		go func() {
			if err := <-dnsErrCh; err != nil {
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
	})

	var unixListener net.Listener
	var tcpListener net.Listener
	if serverSocket != "" {
		listener, err := startUnixSocketListener(serverSocket)
		if err != nil {
			return fmt.Errorf("unix socket listener error: %w", err)
		}
		unixListener = listener
		go acceptInteractiveSessions(listener)
	}
	if serverTCP != "" {
		listener, err := startTCPTerminalListener(serverTCP)
		if err != nil {
			return fmt.Errorf("tcp listener error: %w", err)
		}
		tcpListener = listener
		go acceptInteractiveSessions(listener)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	fmt.Println("Press Ctrl+C to exit daemon mode.")
	<-sigCh
	fmt.Println("Shutting down.")
	stopDNSServer(appState)
	if fullStatsTracker != nil {
		if err := fullStatsTracker.Close(); err != nil {
			log.Printf("Error closing full stats tracker: %v", err)
		}
	}
	if unixListener != nil {
		_ = unixListener.Close()
	}
	if tcpListener != nil {
		_ = tcpListener.Close()
	}
	if serverSocket != "" {
		_ = syscall.Unlink(serverSocket)
	}
	return nil
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
		APIEndpoint:         apiEndpoint,
		APIEnabled:          listener.APIEnabled,
		APIRunning:          state.APIRunning(),
	}
	return info
}

func startAPIAsync(state *daemon.State, port string) {
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
	api.Start(state, trimmed, nil)
}

func startDNSServer(state *daemon.State, port string) (<-chan struct{}, <-chan error) {
	trimmedPort := strings.TrimSpace(port)
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.DNSPort = trimmedPort
	})
	dnsData := data.GetInstance()

	server := &dns.Server{
		Addr: fmt.Sprintf(":%s", trimmedPort),
		Net:  "udp",
	}

	log.Printf("Starting DNS server on %s\n", server.Addr)

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
		if err := server.Shutdown(); err != nil {
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
		fmt.Fprintf(os.Stderr, "Error restarting DNS server: %v\n", err)
	}
}

func stopDNSServer(state *daemon.State) {
	if !state.ServerStatus() {
		return
	}
	stoppedCh := state.SignalStop()
	<-stoppedCh
	state.SetServerStatus(false)
}

func getServerStatus(state *daemon.State) bool {
	return state.ServerStatus()
}

func logQuery(format string, args ...interface{}) {
	if !appState.DaemonMode() {
		return
	}
	fmt.Printf(format, args...)
}

// DNS
func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = false

	dnsData := data.GetInstance()
	dnsData.IncrementTotalQueries()

	// Extract requester IP
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
			// Record full stats if enabled
			if fullStatsTracker != nil {
				recordType := dns.TypeToString[question.Qtype]
				key := fmt.Sprintf("%s:%s", question.Name, recordType)
				if err := fullStatsTracker.RecordRequest(key, requesterIP, recordType); err != nil {
					log.Printf("Error recording full stats: %v", err)
				}
			}

			dnsResolver.HandleQuestion(ctx, question, response)
		}
	}

	err := writer.WriteMsg(response)
	if err != nil {
		log.Println("Error writing response:", err)
	}
}

func startUnixSocketListener(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return nil, nil
	}
	if err := syscall.Unlink(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove unix socket: %w", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on UNIX socket at %s", socketPath)
	return listener, nil
}

func startTCPTerminalListener(address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on TCP address %s for TUI clients", address)
	return listener, nil
}

func acceptInteractiveSessions(listener net.Listener) {
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("Temporary accept error: %v", err)
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("Error accepting connection: %v", err)
			return
		}
		go serveInteractiveSession(conn)
	}
}

func serveInteractiveSession(conn net.Conn) {
	defer conn.Close()

	addr := formatConnAddr(conn)
	log.Printf("TUI client connected: %s", addr)
	defer log.Printf("TUI client disconnected: %s", addr)

	tuiLock := appState.TUISessionMutex()
	tuiLock.Lock()
	defer tuiLock.Unlock()

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(conn, "Error initialising session: %v\r\n", err)
		return
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = writePipe
	os.Stderr = writePipe
	prevOutputWriter := tui.SetOutputWriter(writePipe)

	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&crlfWriter{w: conn}, readPipe)
		close(copyDone)
	}()

	defer func() {
		tui.SetOutputWriter(prevOutputWriter)
		_ = writePipe.Close()
		<-copyDone
		_ = readPipe.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	cfg := appState.ReadlineConfig()
	cfg.Stdin = conn
	cfg.Stdout = conn
	cfg.Stderr = conn
	// Use the configured history file, or default if empty
	if cfg.HistoryFile == "" {
		cfg.HistoryFile = "/tmp/dnsplane.history"
	}
	cfg.DisableAutoSaveHistory = false // Enable auto-save for client sessions
	cfg.FuncMakeRaw = func() error { return nil }
	cfg.FuncExitRaw = func() error { return nil }
	cfg.FuncIsTerminal = func() bool { return true }
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

func connectToInteractiveEndpoint(target string) {
	network, address := resolveInteractiveTarget(target)
	conn, err := net.Dial(network, address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %v\n", address, err)
		return
	}
	defer conn.Close()

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
		_, _ = io.Copy(os.Stdout, conn)
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

	fmt.Println("Connection closed.")
}

func resolveInteractiveTarget(target string) (network, address string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "unix", defaultUnixSocketPath
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
