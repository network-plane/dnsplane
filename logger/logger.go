// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dnsplane/config"

	lj "gopkg.in/natefinch/lumberjack.v2"
)

const defaultAsyncLogQueueSize = 10000

const (
	rotationCheckInterval = 5 * time.Minute
	clientLogFilename     = "dnsplaneclient.log"
)

// Fixed server log file names.
const (
	DNSServerLog = "dnsserver.log"
	APIServerLog = "apiserver.log"
	TUIServerLog = "tuiserver.log"
)

// safeWriter wraps a writer and on write failure falls back to stderr without failing.
type safeWriter struct {
	inner io.Writer
}

func (w *safeWriter) Write(p []byte) (n int, err error) {
	n, err = w.inner.Write(p)
	if err != nil {
		_, _ = os.Stderr.Write([]byte("[log write failed, logging to stderr] "))
		_, _ = os.Stderr.Write(p)
		return len(p), nil // pretend success so caller doesn't abort
	}
	return n, nil
}

// throttleRotateWriter wraps lumberjack and only runs a time-based rotation check every 5m.
type throttleRotateWriter struct {
	lj           *lj.Logger
	lastCheck    time.Time
	mu           sync.Mutex
	maxAgeDays   int
	rotateByTime bool
}

func (w *throttleRotateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	if w.rotateByTime && time.Since(w.lastCheck) > rotationCheckInterval {
		w.lastCheck = time.Now()
		info, err := os.Stat(w.lj.Filename)
		if err == nil && info.ModTime().Before(time.Now().Add(-time.Duration(w.maxAgeDays)*24*time.Hour)) {
			_ = w.lj.Rotate()
		}
	}
	w.mu.Unlock()
	return w.lj.Write(p)
}

// buildLumberjack creates a lumberjack logger for the given path and config.
// For rotation "none", maxSize and maxAge are 0 (no rotation).
func buildLumberjack(logPath string, logCfg config.LogConfig) *lj.Logger {
	rot := &lj.Logger{
		Filename: logPath,
	}
	switch logCfg.Rotation {
	case config.LogRotationSize:
		rot.MaxSize = logCfg.RotationSizeMB
		if rot.MaxSize <= 0 {
			rot.MaxSize = 100
		}
		rot.MaxAge = logCfg.RotationDays
		if rot.MaxBackups == 0 {
			rot.MaxBackups = 3
		}
	case config.LogRotationTime:
		rot.MaxSize = 0
		rot.MaxAge = logCfg.RotationDays
		if rot.MaxAge <= 0 {
			rot.MaxAge = 7
		}
		rot.MaxBackups = 3
	default:
		// none: no rotation, but we still use lumberjack so we have one file
		rot.MaxSize = 0
		rot.MaxAge = 0
		rot.MaxBackups = 0
	}
	return rot
}

// SeverityNone disables logging: no files are created, all output is discarded.
const SeverityNone = "none"

func isSeverityNone(severity string) bool {
	return strings.EqualFold(severity, SeverityNone)
}

// levelFromSeverity maps config severity string to slog.Level.
func levelFromSeverity(severity string) slog.Level {
	switch strings.ToLower(severity) {
	case SeverityNone:
		return slog.LevelError + 1000 // effectively nothing passes
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newFileWriter creates an io.Writer for the given path and log config.
// It creates the directory if needed. On write failure it falls back to stderr (safeWriter).
func newFileWriter(logPath string, logCfg config.LogConfig) (io.Writer, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %s: %w", dir, err)
	}
	rot := buildLumberjack(logPath, logCfg)
	var inner io.Writer = rot
	if logCfg.Rotation == config.LogRotationTime {
		inner = &throttleRotateWriter{
			lj:           rot,
			lastCheck:    time.Time{},
			maxAgeDays:   logCfg.RotationDays,
			rotateByTime: true,
		}
	}
	return &safeWriter{inner: inner}, nil
}

// NewServerLogger creates a slog.Logger that writes to a service-specific log file
// under logDir (e.g. dnsserver.log, apiserver.log, tuiserver.log).
// If severity is "none", no files are created and all log output is discarded.
// If the file cannot be created or written (and severity is not "none"), logs are written to stderr instead.
func NewServerLogger(serviceLogName, logDir string, logCfg config.LogConfig) *slog.Logger {
	if isSeverityNone(logCfg.Severity) {
		h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1000})
		return slog.New(h)
	}
	logPath := filepath.Join(logDir, serviceLogName)
	wr, err := newFileWriter(logPath, logCfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "logger: failed to open %s: %v; using stderr\n", logPath, err)
		wr = os.Stderr
	}
	level := levelFromSeverity(logCfg.Severity)
	h := slog.NewTextHandler(wr, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// ClientLogPath returns the path for the client log file.
// If path is a directory, returns path/dnsplaneclient.log; otherwise returns path as-is.
func ClientLogPath(path string) string {
	path = filepath.Clean(path)
	if path == "" {
		return clientLogFilename
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return filepath.Join(path, clientLogFilename)
	}
	return path
}

// NewClientLogger creates a slog.Logger that writes to the given path (file or dir/dnsplaneclient.log).
// Only used when the user passes --log-file. Uses simple rotation (size 10MB, 3 days) for client.
func NewClientLogger(logFilePath string) *slog.Logger {
	logPath := ClientLogPath(logFilePath)
	cfg := config.LogConfig{
		Dir:            filepath.Dir(logPath),
		Severity:       "info",
		Rotation:       config.LogRotationSize,
		RotationSizeMB: 10,
		RotationDays:   3,
	}
	// Build path for buildLumberjack (we need full path)
	dir := filepath.Dir(logPath)
	if dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	wr, err := newFileWriter(logPath, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "logger: failed to open client log %s: %v; using stderr\n", logPath, err)
		wr = os.Stderr
	}
	h := slog.NewTextHandler(wr, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h)
}

// AsyncLogQueue runs log (and other) callbacks in a single background goroutine
// so the caller never blocks on I/O. Used for the DNS reply path: the reply is
// sent immediately and all logging/stats happen asynchronously.
type AsyncLogQueue struct {
	ch        chan func()
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewAsyncLogQueue creates a queue with the given buffer size and starts the worker.
// If size <= 0, defaultAsyncLogQueueSize is used.
func NewAsyncLogQueue(size int) *AsyncLogQueue {
	if size <= 0 {
		size = defaultAsyncLogQueueSize
	}
	q := &AsyncLogQueue{ch: make(chan func(), size)}
	q.wg.Add(1)
	go q.worker()
	return q
}

func (q *AsyncLogQueue) worker() {
	defer q.wg.Done()
	for f := range q.ch {
		f()
	}
}

// Enqueue adds f to the queue. If the queue is full, f is dropped so the caller never blocks.
func (q *AsyncLogQueue) Enqueue(f func()) {
	if q == nil || q.ch == nil {
		return
	}
	select {
	case q.ch <- f:
	default:
		// Queue full; drop to avoid blocking the DNS path
	}
}

// Close closes the queue and waits for the worker to drain. Idempotent.
func (q *AsyncLogQueue) Close() {
	if q == nil {
		return
	}
	q.closeOnce.Do(func() {
		close(q.ch)
		q.wg.Wait()
	})
}
