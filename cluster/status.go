// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"strings"
	"sync"
	"time"
)

// PeerStatus is runtime observability for one configured cluster peer (outbound dial target).
type PeerStatus struct {
	Address         string     `json:"address"`
	LastOutboundOK  *time.Time `json:"last_outbound_ok,omitempty"`
	LastOutboundErr string     `json:"last_outbound_error,omitempty"`
	LastInboundOK   *time.Time `json:"last_inbound_ok,omitempty"`
	LastPullOK      *time.Time `json:"last_pull_ok,omitempty"`
	LastProbeOK     *time.Time `json:"last_probe_ok,omitempty"`
	LastProbeRTTMs  float64    `json:"last_probe_rtt_ms,omitempty"`
	LastProbeErr    string     `json:"last_probe_error,omitempty"`
	Reachable       bool       `json:"reachable"`
}

// StatusSnapshot is JSON-safe cluster runtime status for dashboard / TUI.
type StatusSnapshot struct {
	Enabled          bool         `json:"enabled"`
	NodeID           string       `json:"node_id"`
	ListenAddr       string       `json:"listen_addr"`
	AdvertiseAddr    string       `json:"advertise_addr,omitempty"`
	ReplicaOnly      bool         `json:"replica_only"`
	ClusterAdmin     bool         `json:"cluster_admin"`
	Peers            []PeerStatus `json:"peers"`
	LocalSeq         uint64       `json:"local_seq"`
	ClusterPortGuess string       `json:"cluster_port_guess,omitempty"`
}

type peerTracker struct {
	mu    sync.Mutex
	peers map[string]*PeerStatus
}

func newPeerTracker() *peerTracker {
	return &peerTracker{peers: make(map[string]*PeerStatus)}
}

func (t *peerTracker) getOrCreateLocked(a string) *PeerStatus {
	p, ok := t.peers[a]
	if !ok {
		p = &PeerStatus{Address: a, Reachable: false}
		t.peers[a] = p
	}
	return p
}

func (t *peerTracker) recordOutbound(addr string, ok bool, errStr string) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.getOrCreateLocked(a)
	now := time.Now()
	if ok {
		p.LastOutboundOK = &now
		p.LastOutboundErr = ""
	} else {
		p.LastOutboundErr = errStr
	}
}

func (t *peerTracker) recordInbound(addr string) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.getOrCreateLocked(a)
	now := time.Now()
	p.LastInboundOK = &now
}

func (t *peerTracker) recordPull(addr string, ok bool) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.getOrCreateLocked(a)
	now := time.Now()
	if ok {
		p.LastPullOK = &now
	}
}

func (t *peerTracker) recordProbe(addr string, ok bool, rttMs float64, errStr string) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.getOrCreateLocked(a)
	now := time.Now()
	if ok {
		p.LastProbeOK = &now
		p.LastProbeRTTMs = rttMs
		p.LastProbeErr = ""
		p.Reachable = true
	} else {
		p.LastProbeErr = errStr
		p.Reachable = false
	}
}

func (t *peerTracker) snapshot(addrs []string) []PeerStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]PeerStatus, 0, len(addrs))
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		p, ok := t.peers[a]
		if !ok {
			out = append(out, PeerStatus{Address: a, Reachable: false})
			continue
		}
		c := *p
		out = append(out, c)
	}
	return out
}
