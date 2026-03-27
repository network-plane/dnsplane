// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestStatsHTMLGates_NoConfig(t *testing.T) {
	if instance != nil {
		t.Skip("resolver already initialised")
	}
	if !StatsPerfPageHTMLEnabled() || !StatsDashboardHTMLEnabled() {
		t.Fatal("expected perf and dashboard HTML gates true when instance and configState nil")
	}
}

func TestStatsPerfPageHTMLEnabled_FromConfigState(t *testing.T) {
	if instance != nil {
		t.Skip("resolver instance already initialised; configState path not tested")
	}
	dir := t.TempDir()
	loaded := &config.Loaded{
		Path: filepath.Join(dir, "dnsplane.json"),
		Config: config.Config{
			FileLocations: config.FileLocations{
				DNSServerFile: filepath.Join(dir, "dnsservers.json"),
			},
			StatsPerfPageEnabled:  false,
			StatsDashboardEnabled: true,
		},
	}
	SetConfig(loaded)
	t.Cleanup(func() {
		configStateMu.Lock()
		configState = nil
		configStateMu.Unlock()
	})

	if StatsPerfPageHTMLEnabled() {
		t.Fatal("expected perf page disabled from config")
	}
	if !StatsDashboardHTMLEnabled() {
		t.Fatal("expected dashboard enabled")
	}
}
