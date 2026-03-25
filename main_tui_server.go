// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dnsplane/daemon"

	"github.com/chzyer/readline"
	tui "github.com/network-plane/planetui"
)

func resetTUIState() {
	if mgr := tui.DefaultEngine().Contexts(); mgr != nil {
		_ = mgr.PopToRoot()
	}
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
	defer func() { _ = listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
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
	defer func() { _ = conn.Close() }()

	addr := formatConnAddr(conn)
	if log != nil {
		log.Info("TUI client connected", "addr", addr)
		defer func() { log.Debug("TUI client disconnected", "addr", addr) }()
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	clientLine, _ := bufio.NewReader(conn).ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	clientLine = strings.TrimSpace(clientLine)

	tuiLock := appState.TUISessionMutex()
	if strings.TrimSpace(clientLine) == tuiClientKillCmd {
		appState.DisconnectCurrentTUIClient()
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

	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err == nil {
		_, _ = fmt.Fprintf(conn, "%s %s\n", tuiBannerPrefix, appVersion)
		_ = conn.SetWriteDeadline(time.Time{})
	}

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
	cfg.FuncGetWidth = func() int { return 80 }
	cfg.ForceUseInteractive = true

	rl, err := readline.NewEx(&cfg)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "Error initialising session: %v\r\n", err)
		return
	}
	defer func() { _ = rl.Close() }()
	defer resetTUIState()
	resetTUIState()

	if err := tui.Run(rl); err != nil {
		_, _ = fmt.Fprintf(conn, "\r\nSession terminated: %v\r\n", err)
	} else {
		_, _ = fmt.Fprint(conn, "\rShutting down session.\r\n")
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
