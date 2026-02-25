//go:build !unix

// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package config

func runningAsRoot() bool {
	return false
}
