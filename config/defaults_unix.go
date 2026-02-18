//go:build unix

package config

import "syscall"

func runningAsRoot() bool {
	return syscall.Geteuid() == 0
}
