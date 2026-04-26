// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dnsplane/logger"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func runClient(cmd *cobra.Command, args []string) error {
	clientTarget := defaultSocketPath
	if len(args) > 0 {
		clientTarget = args[0]
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
	defer func() { _ = conn.Close() }()

	if killOther {
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		_, _ = fmt.Fprintf(conn, "%s\n", tuiClientKillCmd)
		_ = conn.SetWriteDeadline(time.Time{})
	}

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
	if serverVersion != "" && serverVersion != appVersion {
		fmt.Fprintf(os.Stderr, "Warning: version mismatch — server %s, client %s\n", serverVersion, appVersion)
		if clientLogger != nil {
			clientLogger.Warn("version mismatch", "server", serverVersion, "client", appVersion)
		}
	}

	if clientLogger != nil {
		clientLogger.Info("connected", "network", network, "address", address)
	}
	fmt.Printf("Connected to %s %s\n", network, address)

	fd := os.Stdin.Fd()
	stdinFD := -1
	if fd <= uintptr(math.MaxInt) {
		stdinFD = int(fd) // #nosec G115 -- fd bounded by math.MaxInt
	}
	var (
		oldState *term.State
		restored bool
	)
	if stdinFD >= 0 && term.IsTerminal(stdinFD) {
		if st, err := term.MakeRaw(stdinFD); err == nil {
			oldState = st
			defer func() {
				if !restored {
					if err := term.Restore(stdinFD, oldState); err == nil {
						restored = true
					}
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
			if err := term.Restore(stdinFD, oldState); err != nil {
				_ = err
			}
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
