// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestNormalizeSyncPolicy(t *testing.T) {
	if g := NormalizeSyncPolicy(""); g != SyncPolicyLWWPerNode {
		t.Fatalf("empty: got %q", g)
	}
	if g := NormalizeSyncPolicy("PRIMARY_WRITER"); g != SyncPolicyPrimaryWriter {
		t.Fatalf("got %q", g)
	}
	if g := NormalizeSyncPolicy("nope"); g != SyncPolicyLWWPerNode {
		t.Fatalf("unknown: got %q", g)
	}
}

func TestShouldApplySnapshot_primaryWriter(t *testing.T) {
	dir := t.TempDir()
	st := newStateManager(filepath.Join(dir, "dnsplane.json"))
	if err := st.load(""); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		ClusterSyncPolicy:           SyncPolicyPrimaryWriter,
		ClusterAllowedWriterNodeIDs: []string{"writer-a"},
	}
	msg := &RecordsFullMessage{NodeID: "other", Seq: 1}
	ok, reason := ShouldApplySnapshot(cfg, st, msg)
	if ok || reason != "primary_writer_denied" {
		t.Fatalf("got ok=%v reason=%q", ok, reason)
	}
	msg.NodeID = "writer-a"
	ok, reason = ShouldApplySnapshot(cfg, st, msg)
	if !ok || reason != "" {
		t.Fatalf("got ok=%v reason=%q", ok, reason)
	}
	cfg.ClusterAllowedWriterNodeIDs = nil
	ok, reason = ShouldApplySnapshot(cfg, st, msg)
	if ok || reason != "primary_writer_no_allowlist" {
		t.Fatalf("got ok=%v reason=%q", ok, reason)
	}
}

func TestShouldApplySnapshot_globalLWW(t *testing.T) {
	dir := t.TempDir()
	st := newStateManager(filepath.Join(dir, "dnsplane.json"))
	if err := st.load(""); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ClusterSyncPolicy: SyncPolicyGlobalLWW}
	msg := &RecordsFullMessage{NodeID: "a", Seq: 1, Timestamp: 100}
	ok, reason := ShouldApplySnapshot(cfg, st, msg)
	if !ok {
		t.Fatalf("first: ok=%v reason=%q", ok, reason)
	}
	st.commitGlobalLWW(100, "a")
	msg.Timestamp = 99
	msg.Seq = 2
	ok, reason = ShouldApplySnapshot(cfg, st, msg)
	if ok || reason != "global_lww_stale" {
		t.Fatalf("stale: ok=%v reason=%q", ok, reason)
	}
	msg.Timestamp = 100
	msg.NodeID = "b"
	ok, reason = ShouldApplySnapshot(cfg, st, msg)
	if !ok || reason != "" {
		t.Fatalf("tie-break node b > a: ok=%v reason=%q", ok, reason)
	}
}
