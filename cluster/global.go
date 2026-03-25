// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import "sync"

var (
	globalMu      sync.RWMutex
	globalManager *Manager
)

// SetGlobalManager registers the cluster manager for TUI and diagnostics.
func SetGlobalManager(m *Manager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalManager = m
}

// GlobalManager returns the registered manager or nil.
func GlobalManager() *Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalManager
}
