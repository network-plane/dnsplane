// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package logger

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"dnsplane/config"
)

func TestNewServerLogger_SeverityNoneErrorsToStderr(t *testing.T) {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	var wg sync.WaitGroup
	wg.Add(1)
	buf := bytes.Buffer{}
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()

	log := NewServerLogger(DNSServerLog, t.TempDir(), config.LogConfig{Severity: "none"})
	log.Error("startup failed", "reason", "test")

	_ = w.Close()
	os.Stderr = old
	wg.Wait()

	if !bytes.Contains(buf.Bytes(), []byte("startup failed")) {
		t.Fatalf("stderr missing error: %q", buf.Bytes())
	}
}

func TestStderrTeeHandlerDuplicatesErrors(t *testing.T) {
	var fileBuf bytes.Buffer
	primary := slog.NewTextHandler(&fileBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	stderrErr := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})
	// use a real tee with stderr to pipe in integration test above; here only check primary gets info
	log := slog.New(&stderrTeeHandler{primary: primary, stderr: stderrErr})
	log.Info("hello")
	log.Error("boom")
	if !bytes.Contains(fileBuf.Bytes(), []byte("boom")) {
		t.Fatalf("primary should log error: %q", fileBuf.Bytes())
	}
}