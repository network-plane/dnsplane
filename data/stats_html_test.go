// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestStatsPageHTMLEnabled_NoConfig(t *testing.T) {
	if instance != nil {
		t.Skip("resolver already initialised")
	}
	if !StatsPageHTMLEnabled() || !StatsPerfPageHTMLEnabled() || !StatsDashboardHTMLEnabled() {
		t.Fatal("expected all true when instance and configState nil")
	}
}

func TestStatsPageHTMLEnabled_FromConfigState(t *testing.T) {
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
			StatsPageEnabled:      false,
			StatsPerfPageEnabled:  true,
			StatsDashboardEnabled: true,
		},
	}
	SetConfig(loaded)
	t.Cleanup(func() {
		configStateMu.Lock()
		configState = nil
		configStateMu.Unlock()
	})

	if StatsPageHTMLEnabled() {
		t.Fatal("expected stats page disabled from config")
	}
	if !StatsPerfPageHTMLEnabled() {
		t.Fatal("expected perf page enabled")
	}
}
