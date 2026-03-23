// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"dnsplane/abuse"
	"dnsplane/config"
	"dnsplane/data"
	"dnsplane/dnsserve"
	"dnsplane/resolver"

	"github.com/miekg/dns"
)

var responseLimiter dnsserve.ResponseLimiter

func buildResponseLimiter(st config.Config) dnsserve.ResponseLimiter {
	mode := strings.ToLower(strings.TrimSpace(st.DNSResponseLimitMode))
	if mode == "" {
		mode = "sliding_window"
	}
	if mode == "rrl" {
		if st.DNSRRLMaxPerBucket <= 0 {
			return nil
		}
		win := time.Duration(st.DNSRRLWindowSeconds) * time.Second
		if win <= 0 {
			win = time.Second
		}
		slip := st.DNSRRLSlip
		if slip <= 0 {
			slip = 0.1
		}
		return abuse.NewRRL(st.DNSRRLMaxPerBucket, win, slip, 0)
	}
	if st.DNSMaxResponsesPerIPWindow <= 0 {
		return nil
	}
	wsec := st.DNSSlidingWindowSeconds
	if wsec <= 0 {
		wsec = 1
	}
	return abuse.NewSlidingWindow(time.Duration(wsec)*time.Second, st.DNSMaxResponsesPerIPWindow, 0)
}

func handleRequestProto(w dns.ResponseWriter, request *dns.Msg, proto string) {
	_ = proto // reserved for metrics / future per-protocol stats
	t0 := time.Now()
	requesterIP := "unknown"
	if addr := w.RemoteAddr(); addr != nil {
		if host, _, err := net.SplitHostPort(addr.String()); err == nil {
			requesterIP = host
		} else {
			requesterIP = addr.String()
		}
	}

	ctx := resolver.ContextWithRequest(context.Background(), request)
	dep := dnsserve.Dependencies{
		Resolver:        dnsResolver,
		Settings:        func() config.Config { return data.GetInstance().GetResolverSettings() },
		ResponseLimiter: responseLimiter,
		QueryLimiter:    dnsQueryLimiter,
		OnLimiterDrop:   data.RecordLimiterDrop,
	}
	response := dnsserve.ServeDNS(ctx, request, dnsserve.ServeMeta{ClientIP: requesterIP, Protocol: proto}, dep)
	resolveDone := time.Since(t0)

	err := w.WriteMsg(response)
	total := time.Since(t0)

	if total > 10*time.Millisecond {
		qname := ""
		if len(request.Question) > 0 {
			qname = request.Question[0].Name
		}
		fmt.Printf("[SLOW] %s resolve=%s write=%s total=%s\n", qname, resolveDone, total-resolveDone, total)
	}

	go func() {
		dnsData := data.GetInstance()
		dnsData.IncrementTotalQueries()
		if err != nil && asyncLogQueue != nil && dnsLogger != nil {
			errCopy := err
			asyncLogQueue.Enqueue(func() { dnsLogger.Error("Error writing response", "error", errCopy) })
		}
	}()
}

func startInboundDNSListeners(stopCh <-chan struct{}) {
	st := data.GetInstance().GetResolverSettings()
	if st.DOTEnabled && strings.TrimSpace(st.DOTCertFile) != "" && strings.TrimSpace(st.DOTKeyFile) != "" {
		go runDotServer(stopCh, st)
	}
	if st.DOHEnabled && strings.TrimSpace(st.DOHCertFile) != "" && strings.TrimSpace(st.DOHKeyFile) != "" {
		go runDoHServer(stopCh, st)
	}
}

func runDotServer(stopCh <-chan struct{}, st config.Config) {
	cert, err := tls.LoadX509KeyPair(st.DOTCertFile, st.DOTKeyFile)
	if err != nil {
		if dnsLogger != nil {
			dnsLogger.Error("DoT TLS load failed", "error", err)
		}
		return
	}
	port := strings.TrimSpace(st.DOTPort)
	if port == "" {
		port = "853"
	}
	bind := strings.TrimSpace(st.DOTBind)
	addr := ":" + port
	if bind != "" {
		addr = net.JoinHostPort(bind, port)
	}
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequestProto(w, r, dnsserve.ProtoDoT)
	})
	srv := &dns.Server{
		Addr:         addr,
		Net:          "tcp-tls",
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if dnsLogger != nil {
		dnsLogger.Info("Starting DoT listener", "addr", addr)
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && dnsLogger != nil {
			dnsLogger.Error("DoT server stopped", "error", err)
		}
	}()
	go func() {
		<-stopCh
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.ShutdownContext(shutdownCtx)
	}()
}

func runDoHServer(stopCh <-chan struct{}, st config.Config) {
	port := strings.TrimSpace(st.DOHPort)
	if port == "" {
		port = "8443"
	}
	path := strings.TrimSpace(st.DOHPath)
	if path == "" {
		path = "/dns-query"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	bind := strings.TrimSpace(st.DOHBind)
	addr := ":" + port
	if bind != "" {
		addr = net.JoinHostPort(bind, port)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		doHHandler(w, r, path)
	})
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if dnsLogger != nil {
		dnsLogger.Info("Starting DoH listener", "addr", addr, "path", path)
	}
	go func() {
		if err := srv.ListenAndServeTLS(st.DOHCertFile, st.DOHKeyFile); err != nil && err != http.ErrServerClosed && dnsLogger != nil {
			dnsLogger.Error("DoH server stopped", "error", err)
		}
	}()
	go func() {
		<-stopCh
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

func doHHandler(w http.ResponseWriter, r *http.Request, path string) {
	if r.URL.Path != path {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var wire []byte
	var err error
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query().Get("dns")
		if q == "" {
			http.Error(w, "missing dns query parameter", http.StatusBadRequest)
			return
		}
		wire, err = base64.RawURLEncoding.DecodeString(q)
		if err != nil {
			http.Error(w, "invalid dns parameter", http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/dns-message") {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		const maxBody = 65535
		wire, err = io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}
	if len(wire) < 12 {
		http.Error(w, "message too short", http.StatusBadRequest)
		return
	}
	req := new(dns.Msg)
	if err := req.Unpack(wire); err != nil {
		http.Error(w, "invalid DNS message", http.StatusBadRequest)
		return
	}
	requesterIP := "unknown"
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		requesterIP = host
	} else {
		requesterIP = r.RemoteAddr
	}
	ctx := resolver.ContextWithRequest(context.Background(), req)
	dep := dnsserve.Dependencies{
		Resolver:        dnsResolver,
		Settings:        func() config.Config { return data.GetInstance().GetResolverSettings() },
		ResponseLimiter: responseLimiter,
		QueryLimiter:    dnsQueryLimiter,
		OnLimiterDrop:   data.RecordLimiterDrop,
	}
	resp := dnsserve.ServeDNS(ctx, req, dnsserve.ServeMeta{ClientIP: requesterIP, Protocol: dnsserve.ProtoDoH}, dep)
	out, err := resp.Pack()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/dns-message")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(out)

	go func() {
		dnsData := data.GetInstance()
		dnsData.IncrementTotalQueries()
	}()
}
