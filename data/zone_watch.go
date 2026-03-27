// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const bindDirWatchDebounce = 500 * time.Millisecond

// startBindDirWatch watches zoneDir for changes and reloads records after a short debounce.
// The returned function stops the watcher (idempotent).
func startBindDirWatch(d *DNSResolverData, zoneDir string) func() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		resolverSlog().Error("bind_dir watch: fsnotify init failed", "error", err)
		return func() {}
	}
	if err := watcher.Add(zoneDir); err != nil {
		resolverSlog().Error("bind_dir watch: add path failed", "path", zoneDir, "error", err)
		_ = watcher.Close()
		return func() {}
	}

	done := make(chan struct{})
	var mu sync.Mutex
	var pending *time.Timer

	schedule := func() {
		mu.Lock()
		defer mu.Unlock()
		if pending != nil {
			pending.Stop()
		}
		pending = time.AfterFunc(bindDirWatchDebounce, func() {
			n, err := d.ReloadRecordsFromSource()
			if err != nil {
				resolverSlog().Warn("bind_dir watch: reload failed", "error", err)
				return
			}
			resolverSlog().Info("bind_dir watch: reload completed", "records", n)
		})
	}

	go func() {
		for {
			select {
			case <-done:
				mu.Lock()
				if pending != nil {
					pending.Stop()
				}
				mu.Unlock()
				_ = watcher.Close()
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) || ev.Has(fsnotify.Rename) || ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Chmod) {
					schedule()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if err != nil {
					resolverSlog().Warn("bind_dir watch: watcher error", "error", err)
				}
			}
		}
	}()

	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
}
