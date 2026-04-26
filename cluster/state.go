// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PersistedState is stored next to dnsplane.json as cluster_state.json.
type PersistedState struct {
	NodeID              string            `json:"node_id"`
	LocalSeq            uint64            `json:"local_seq"`
	PeerSeq             map[string]uint64 `json:"peer_seq,omitempty"`
	LastGlobalLWWUnix   int64             `json:"last_global_lww_unix,omitempty"`
	LastGlobalLWWNodeID string            `json:"last_global_lww_node_id,omitempty"`
}

type stateManager struct {
	mu                  sync.Mutex
	path                string
	nodeID              string
	localSeq            uint64
	peerSeq             map[string]uint64
	lastGlobalLWWUnix   int64
	lastGlobalLWWNodeID string
}

func newStateManager(configFilePath string) *stateManager {
	dir := filepath.Dir(configFilePath)
	return &stateManager{
		path:    filepath.Join(dir, "cluster_state.json"),
		peerSeq: make(map[string]uint64),
	}
}

func (s *stateManager) load(cfgNodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path) // #nosec G304 -- path beside operator config (cluster_state.json)
	if err != nil {
		if os.IsNotExist(err) {
			s.nodeID = strings.TrimSpace(cfgNodeID)
			if s.nodeID == "" {
				s.nodeID = "node-" + randomID()
			}
			s.localSeq = 0
			s.peerSeq = make(map[string]uint64)
			return s.saveLocked()
		}
		return err
	}
	var ps PersistedState
	if err := json.Unmarshal(data, &ps); err != nil {
		return err
	}
	s.nodeID = strings.TrimSpace(ps.NodeID)
	if s.nodeID == "" {
		s.nodeID = "node-" + randomID()
	}
	if strings.TrimSpace(cfgNodeID) != "" {
		s.nodeID = strings.TrimSpace(cfgNodeID)
	}
	s.localSeq = ps.LocalSeq
	s.peerSeq = ps.PeerSeq
	if s.peerSeq == nil {
		s.peerSeq = make(map[string]uint64)
	}
	s.lastGlobalLWWUnix = ps.LastGlobalLWWUnix
	s.lastGlobalLWWNodeID = strings.TrimSpace(ps.LastGlobalLWWNodeID)
	return nil
}

func (s *stateManager) saveLocked() error {
	ps := PersistedState{
		NodeID:              s.nodeID,
		LocalSeq:            s.localSeq,
		PeerSeq:             s.peerSeq,
		LastGlobalLWWUnix:   s.lastGlobalLWWUnix,
		LastGlobalLWWNodeID: s.lastGlobalLWWNodeID,
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *stateManager) NodeID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nodeID
}

func (s *stateManager) NextLocalSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localSeq++
	_ = s.saveLocked()
	return s.localSeq
}

// CurrentLocalSeq returns the sequence number of the last local write (no increment).
func (s *stateManager) CurrentLocalSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.localSeq
}

// IsDuplicate returns true if this peer seq was already applied.
// Seq 0 is normalized to 1 for comparison (empty snapshots use seq 0 in JSON).
func (s *stateManager) IsDuplicate(peerNodeID string, seq uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if peerNodeID == "" {
		return true
	}
	if seq == 0 {
		seq = 1
	}
	return seq <= s.peerSeq[peerNodeID]
}

// CommitPeerSeq records a successful apply from peer.
func (s *stateManager) CommitPeerSeq(peerNodeID string, seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if peerNodeID == "" {
		return
	}
	if seq == 0 {
		seq = 1
	}
	if seq > s.peerSeq[peerNodeID] {
		s.peerSeq[peerNodeID] = seq
		_ = s.saveLocked()
	}
}

func randomID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// shouldAcceptGlobalLWW returns true if (ts, nodeID) wins over the last applied global LWW tuple.
func (s *stateManager) shouldAcceptGlobalLWW(ts int64, nodeID string) bool {
	if ts <= 0 {
		ts = 1
	}
	nodeID = strings.TrimSpace(nodeID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastGlobalLWWUnix == 0 && s.lastGlobalLWWNodeID == "" {
		return true
	}
	if ts > s.lastGlobalLWWUnix {
		return true
	}
	if ts < s.lastGlobalLWWUnix {
		return false
	}
	return nodeID > s.lastGlobalLWWNodeID
}

func (s *stateManager) commitGlobalLWW(ts int64, nodeID string) {
	if ts <= 0 {
		ts = 1
	}
	nodeID = strings.TrimSpace(nodeID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGlobalLWWUnix = ts
	s.lastGlobalLWWNodeID = nodeID
	_ = s.saveLocked()
}

// GlobalLWWLast returns the persisted global LWW winner (for status).
func (s *stateManager) GlobalLWWLast() (ts int64, nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastGlobalLWWUnix, s.lastGlobalLWWNodeID
}
