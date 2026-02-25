//go:build unix
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package config

import "syscall"

func runningAsRoot() bool {
	return syscall.Geteuid() == 0
}
