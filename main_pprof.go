// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	_ "net/http/pprof" // registers /debug/pprof/* on DefaultServeMux
)

var (
	pprofMu     sync.Mutex
	pprofServer *http.Server
)

// startPprof starts an HTTP server on addr serving /debug/pprof/* (CPU, heap, mutex, goroutine, etc.).
// addr must be non-empty (caller should set default e.g. 127.0.0.1:6060 when enabled).
func startPprof(addr string, logger *slog.Logger) {
	pprofMu.Lock()
	defer pprofMu.Unlock()
	if pprofServer != nil {
		return
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	pprofServer = srv
	go func() {
		if logger != nil {
			logger.Info("pprof HTTP server listening (go tool pprof http://HOST/debug/pprof/)", "addr", addr)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if logger != nil {
				logger.Error("pprof server exited", "error", err)
			}
		}
	}()
}

func stopPprof() {
	pprofMu.Lock()
	srv := pprofServer
	pprofServer = nil
	pprofMu.Unlock()
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
