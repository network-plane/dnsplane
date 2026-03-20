// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package commandhandler

import (
	"encoding/json"
	"fmt"
	"strings"

	"dnsplane/cliutil"
	"dnsplane/cluster"
	"dnsplane/data"

	tui "github.com/network-plane/planetui"
)

func runCluster() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		args := input.Raw
		if cliutil.IsHelpRequest(args) {
			return tui.CommandResult{
				Status:   tui.StatusSuccess,
				Messages: infoMessages(clusterHelpLines()...),
			}
		}
		if len(args) == 0 {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Message: "cluster: use cluster status | pull | join | peer ... | push ...", Severity: tui.SeverityWarning},
			}
		}
		mgr := cluster.GlobalManager()
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "status":
			if mgr == nil {
				return failCluster("cluster manager not available")
			}
			snap := mgr.StatusSnapshot()
			b, _ := json.MarshalIndent(snap, "", "  ")
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages(string(b))}
		case "pull", "sync":
			if mgr == nil {
				return failCluster("cluster manager not available")
			}
			mgr.ForcePull()
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: pull triggered")}
		case "join", "info":
			if mgr == nil {
				return failCluster("cluster manager not available")
			}
			nodeID, listen, dial, fp := mgr.JoinInfo()
			lines := []string{
				"Join / cluster identity (copy dial address to the full server TUI):",
				fmt.Sprintf("  node_id:           %s", nodeID),
				fmt.Sprintf("  listen_addr:       %s", listen),
				fmt.Sprintf("  dial_address:      %s", dial),
				fmt.Sprintf("  token_sha256_hex:  %s", fp),
				"  (Verify token fingerprint matches the peer after setting cluster_auth_token.)",
			}
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages(lines...)}
		case "peer":
			return handleClusterPeer(args[1:], mgr)
		case "push":
			return handleClusterPush(args[1:], mgr)
		default:
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Message: "cluster: unknown subcommand (try cluster ?)", Severity: tui.SeverityWarning},
			}
		}
	}
}

func clusterHelpLines() []string {
	return []string{
		"cluster status              — JSON cluster runtime status",
		"cluster pull                — pull snapshots from all peers",
		"cluster join                — show node_id, dial address, token fingerprint",
		"cluster peer add <host:port> [full|readonly] — add peer locally; optional role push (admin)",
		"cluster peer remove <host:port> — remove from local config",
		"cluster peer set-role <host:port> full|readonly — remote admin (cluster_admin + cluster_admin_token)",
		"cluster push records <host:port> — push full snapshot to one peer",
		"cluster push config <host:port> — push cluster_auth_token + cluster_peers to peer",
	}
}

func failCluster(msg string) tui.CommandResult {
	return tui.CommandResult{
		Status: tui.StatusFailed,
		Error:  &tui.CommandError{Message: msg, Severity: tui.SeverityError},
	}
}

func handleClusterPeer(args []string, mgr *cluster.Manager) tui.CommandResult {
	if len(args) < 1 {
		return failCluster("cluster peer: need add|remove|set-role")
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "add":
		if len(args) < 2 {
			return failCluster("cluster peer add <host:port> [full|readonly]")
		}
		addr := strings.TrimSpace(args[1])
		role := ""
		if len(args) >= 3 {
			role = strings.ToLower(strings.TrimSpace(args[2]))
			if role != "full" && role != "readonly" {
				return failCluster("role must be full or readonly")
			}
		}
		dnsData := data.GetInstance()
		s := dnsData.GetResolverSettings()
		for _, p := range s.ClusterPeers {
			if strings.TrimSpace(p) == addr {
				return failCluster("peer already in cluster_peers")
			}
		}
		s.ClusterPeers = append(s.ClusterPeers, addr)
		dnsData.UpdateSettings(s)
		if mgr != nil && role != "" {
			replica := role == "readonly"
			err := mgr.AdminPushConfig(addr, cluster.AdminConfigApplyMessage{ReplicaOnly: &replica})
			if err != nil {
				return failCluster(fmt.Sprintf("peer added locally; remote set-role failed: %v", err))
			}
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: peer added and persisted")}
	case "remove":
		if len(args) < 2 {
			return failCluster("cluster peer remove <host:port>")
		}
		addr := strings.TrimSpace(args[1])
		dnsData := data.GetInstance()
		s := dnsData.GetResolverSettings()
		var out []string
		for _, p := range s.ClusterPeers {
			if strings.TrimSpace(p) == addr {
				continue
			}
			out = append(out, p)
		}
		if len(out) == len(s.ClusterPeers) {
			return failCluster("peer not found in cluster_peers")
		}
		s.ClusterPeers = out
		dnsData.UpdateSettings(s)
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: peer removed")}
	case "set-role":
		if len(args) < 3 {
			return failCluster("cluster peer set-role <host:port> full|readonly")
		}
		if mgr == nil {
			return failCluster("cluster manager not available")
		}
		addr := strings.TrimSpace(args[1])
		role := strings.ToLower(strings.TrimSpace(args[2]))
		replica := role == "readonly"
		if role != "full" && role != "readonly" {
			return failCluster("role must be full or readonly")
		}
		err := mgr.AdminPushConfig(addr, cluster.AdminConfigApplyMessage{ReplicaOnly: &replica})
		if err != nil {
			return failCluster(err.Error())
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: peer role updated on remote")}
	default:
		return failCluster("cluster peer: unknown subcommand")
	}
}

func handleClusterPush(args []string, mgr *cluster.Manager) tui.CommandResult {
	if len(args) < 2 {
		return failCluster("cluster push records|config <host:port>")
	}
	if mgr == nil {
		return failCluster("cluster manager not available")
	}
	kind := strings.ToLower(strings.TrimSpace(args[0]))
	addr := strings.TrimSpace(args[1])
	switch kind {
	case "records":
		if err := mgr.PushRecordsToPeer(addr); err != nil {
			return failCluster(err.Error())
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: push records sent")}
	case "config":
		dnsData := data.GetInstance()
		full := dnsData.GetResolverSettings()
		apply := cluster.AdminConfigApplyMessage{
			AuthToken: full.ClusterAuthToken,
			Peers:     append([]string(nil), full.ClusterPeers...),
		}
		if err := mgr.AdminPushConfig(addr, apply); err != nil {
			return failCluster(err.Error())
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("cluster: config pushed to peer")}
	default:
		return failCluster("cluster push: use records or config")
	}
}
