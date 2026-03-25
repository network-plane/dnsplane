// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"dnsplane/daemon"
	"dnsplane/data"
	"dnsplane/dnsserve"

	"github.com/miekg/dns"
)

func dnsListenAddr(port string) string {
	p := strings.TrimSpace(port)
	if p == "" {
		p = "53"
	}
	bind := strings.TrimSpace(data.GetInstance().GetResolverSettings().DNSBind)
	if bind == "" {
		return ":" + p
	}
	return net.JoinHostPort(bind, p)
}

func startDNSServer(state *daemon.State, port string) (<-chan struct{}, <-chan error) {
	trimmedPort := strings.TrimSpace(port)
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.DNSPort = trimmedPort
	})
	dnsData := data.GetInstance()
	dnsAddr := dnsListenAddr(trimmedPort)

	udpMux := dns.NewServeMux()
	udpMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequestProto(w, r, dnsserve.ProtoUDP)
	})
	tcpMux := dns.NewServeMux()
	tcpMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequestProto(w, r, dnsserve.ProtoTCP)
	})

	udpServer := &dns.Server{Addr: dnsAddr, Net: "udp", Handler: udpMux, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	tcpServer := &dns.Server{Addr: dnsAddr, Net: "tcp", Handler: tcpMux, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}

	dnsLogger.Info("Starting DNS servers", "udp", udpServer.Addr, "tcp", tcpServer.Addr)

	startedCh := make(chan struct{})
	errCh := make(chan error, 2)
	var once sync.Once
	udpServer.NotifyStartedFunc = func() {
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
		if err := udpServer.ListenAndServe(); err != nil {
			state.SetServerStatus(false)
			select {
			case errCh <- err:
			default:
			}
			return
		}
		state.SetServerStatus(false)
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			state.SetServerStatus(false)
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	go func() {
		<-state.StopChannel()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := udpServer.ShutdownContext(shutdownCtx); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
		if err := tcpServer.ShutdownContext(shutdownCtx); err != nil {
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
		return
	}
	responseLimiter = buildResponseLimiter(data.GetInstance().GetResolverSettings())
	go startInboundDNSListeners(state.StopChannel())
}

const dnsShutdownWaitTimeout = 20 * time.Second

func stopDNSServer(state *daemon.State) {
	if !state.ServerStatus() {
		return
	}
	stoppedCh := state.SignalStop()
	select {
	case <-stoppedCh:
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
