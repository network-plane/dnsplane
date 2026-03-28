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
	if !StatsDashboardHTMLEnabled() {
		t.Fatal("expected dashboard HTML gate true when instance and configState nil")
	}
}

func TestStatsDashboardHTMLEnabled_FromConfigState(t *testing.T) {
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
			StatsDashboardEnabled: false,
		},
	}
	SetConfig(loaded)
	t.Cleanup(func() {
		configStateMu.Lock()
		configState = nil
		configStateMu.Unlock()
	})

	if StatsDashboardHTMLEnabled() {
		t.Fatal("expected dashboard disabled from config")
	}
}
