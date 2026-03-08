// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestIsSeverityNone(t *testing.T) {
	tests := []struct {
		severity string
		want     bool
	}{
		{"none", true},
		{"None", true},
		{"NONE", true},
		{"", false},
		{"debug", false},
		{"info", false},
	}
	for _, tt := range tests {
		got := isSeverityNone(tt.severity)
		if got != tt.want {
			t.Errorf("isSeverityNone(%q) = %v, want %v", tt.severity, got, tt.want)
		}
	}
}

func TestLevelFromSeverity(t *testing.T) {
	tests := []struct {
		severity string
		want     slog.Level
	}{
		{"none", slog.LevelError + 1000},
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
	}
	for _, tt := range tests {
		got := levelFromSeverity(tt.severity)
		if got != tt.want {
			t.Errorf("levelFromSeverity(%q) = %v, want %v", tt.severity, got, tt.want)
		}
	}
}

func TestClientLogPath(t *testing.T) {
	// Empty path returns default filename only
	got := ClientLogPath("")
	if got != clientLogFilename {
		t.Errorf("ClientLogPath(%q) = %q, want %q", "", got, clientLogFilename)
	}

	// Path to existing directory: should return dir/dnsplaneclient.log
	dir := t.TempDir()
	got = ClientLogPath(dir)
	want := filepath.Join(dir, clientLogFilename)
	if got != want {
		t.Errorf("ClientLogPath(%q) = %q, want %q", dir, got, want)
	}

	// Path to file (non-existent): return path as-is (cleaned)
	got = ClientLogPath("/some/path/client.log")
	if got != "/some/path/client.log" {
		t.Errorf("ClientLogPath(file) = %q, want /some/path/client.log", got)
	}

	// Path to existing file: return path as-is
	f, err := os.CreateTemp(t.TempDir(), "*.log")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	got = ClientLogPath(f.Name())
	if got != f.Name() {
		t.Errorf("ClientLogPath(existing file) = %q, want %q", got, f.Name())
	}
}

func TestBuildLumberjack(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")

	t.Run("rotation none", func(t *testing.T) {
		lj := buildLumberjack(logPath, config.LogConfig{Rotation: config.LogRotationNone})
		if lj.Filename != logPath {
			t.Errorf("Filename = %q, want %q", lj.Filename, logPath)
		}
		if lj.MaxSize != 0 || lj.MaxAge != 0 {
			t.Errorf("rotation none: MaxSize=%d MaxAge=%d, want 0", lj.MaxSize, lj.MaxAge)
		}
	})

	t.Run("rotation size", func(t *testing.T) {
		lj := buildLumberjack(logPath, config.LogConfig{
			Rotation:       config.LogRotationSize,
			RotationSizeMB: 50,
			RotationDays:   5,
		})
		if lj.MaxSize != 50 {
			t.Errorf("MaxSize = %d, want 50", lj.MaxSize)
		}
		if lj.MaxAge != 5 {
			t.Errorf("MaxAge = %d, want 5", lj.MaxAge)
		}
		if lj.MaxBackups == 0 {
			t.Error("MaxBackups should be set for size rotation")
		}
	})

	t.Run("rotation size default when zero", func(t *testing.T) {
		lj := buildLumberjack(logPath, config.LogConfig{Rotation: config.LogRotationSize, RotationSizeMB: 0})
		if lj.MaxSize != 100 {
			t.Errorf("MaxSize with 0 = %d, want default 100", lj.MaxSize)
		}
	})

	t.Run("rotation time", func(t *testing.T) {
		lj := buildLumberjack(logPath, config.LogConfig{
			Rotation:     config.LogRotationTime,
			RotationDays: 7,
		})
		if lj.MaxSize != 0 {
			t.Errorf("rotation time: MaxSize = %d, want 0", lj.MaxSize)
		}
		if lj.MaxAge != 7 {
			t.Errorf("MaxAge = %d, want 7", lj.MaxAge)
		}
	})
}

func TestNewServerLogger_SeverityNone(t *testing.T) {
	// Severity "none" must not create files; logger should discard
	logDir := t.TempDir()
	cfg := config.LogConfig{Severity: "none", Rotation: config.LogRotationNone}
	log := NewServerLogger(DNSServerLog, logDir, cfg)
	if log == nil {
		t.Fatal("NewServerLogger returned nil")
	}
	// No file should exist under logDir
	entries, _ := os.ReadDir(logDir)
	if len(entries) != 0 {
		t.Errorf("severity none: expected no files in log dir, got %d", len(entries))
	}
}
