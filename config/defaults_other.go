//go:build !unix

package config

func runningAsRoot() bool {
	return false
}
