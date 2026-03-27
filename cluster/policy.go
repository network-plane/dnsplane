// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"strings"

	"dnsplane/config"
)

// Sync policy values for cluster_sync_policy (JSON / config).
const (
	SyncPolicyLWWPerNode    = "lww_per_node"
	SyncPolicyPrimaryWriter = "primary_writer"
	SyncPolicyGlobalLWW     = "global_lww"
)

// NormalizeSyncPolicy returns a known policy string; empty or unknown maps to lww_per_node.
func NormalizeSyncPolicy(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", SyncPolicyLWWPerNode:
		return SyncPolicyLWWPerNode
	case SyncPolicyPrimaryWriter, SyncPolicyGlobalLWW:
		return s
	default:
		return SyncPolicyLWWPerNode
	}
}

// WriterAllowed reports whether nodeID is in the allowlist (trimmed element match).
func WriterAllowed(allowed []string, nodeID string) bool {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return false
	}
	for _, id := range allowed {
		if strings.TrimSpace(id) == nodeID {
			return true
		}
	}
	return false
}

// ShouldApplySnapshot decides whether to apply a records_full message (after duplicate-seq check).
// If duplicate is true, the caller should skip without calling this.
func ShouldApplySnapshot(cfg config.Config, st *stateManager, msg *RecordsFullMessage) (ok bool, reason string) {
	pol := NormalizeSyncPolicy(cfg.ClusterSyncPolicy)
	switch pol {
	case SyncPolicyLWWPerNode:
		return true, ""
	case SyncPolicyPrimaryWriter:
		ids := cfg.ClusterAllowedWriterNodeIDs
		if len(ids) == 0 {
			return false, "primary_writer_no_allowlist"
		}
		if !WriterAllowed(ids, msg.NodeID) {
			return false, "primary_writer_denied"
		}
		return true, ""
	case SyncPolicyGlobalLWW:
		ts := msg.Timestamp
		if ts <= 0 {
			ts = 1
		}
		if !st.shouldAcceptGlobalLWW(ts, msg.NodeID) {
			return false, "global_lww_stale"
		}
		return true, ""
	default:
		return true, ""
	}
}
