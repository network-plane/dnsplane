// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

// Package cluster implements multi-node DNS record sync over TCP (length-prefixed JSON).
// Cache and per-node state are not replicated. Use a single writer or accept LWW by sequence.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"dnsplane/config"
	"dnsplane/data"
)

// Manager runs the cluster listener and coordinates pushes.
type Manager struct {
	configPath string
	state      *stateManager
	log        *slog.Logger
	dns        *data.DNSResolverData

	mu       sync.Mutex
	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewManager creates a cluster manager. configPath is the path to dnsplane.json (for cluster_state.json dir).
func NewManager(configPath string, dns *data.DNSResolverData, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		configPath: configPath,
		state:      newStateManager(configPath),
		log:        log,
		dns:        dns,
	}
}

// Start loads state and starts the TCP listener and optional periodic sync.
func (m *Manager) Start(ctx context.Context, cfg config.Config) error {
	if err := m.state.load(cfg.ClusterNodeID); err != nil {
		return fmt.Errorf("cluster state: %w", err)
	}
	if !cfg.ClusterEnabled {
		return nil
	}
	addr := strings.TrimSpace(cfg.ClusterListenAddr)
	if addr == "" {
		addr = ":7946"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cluster listen %s: %w", addr, err)
	}
	m.mu.Lock()
	m.listener = ln
	ctx2, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.log.Info("cluster: listening", "addr", addr, "node_id", m.state.NodeID())

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx2.Done():
					return
				default:
					if m.log != nil {
						m.log.Warn("cluster: accept", "error", err)
					}
					time.Sleep(time.Second)
					continue
				}
			}
			go m.serveConn(ctx2, conn)
		}
	}()

	interval := cfg.ClusterSyncIntervalSeconds
	if interval > 0 {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			t := time.NewTicker(time.Duration(interval) * time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx2.Done():
					return
				case <-t.C:
					m.PullFromPeers(cfg)
				}
			}
		}()
	}

	return nil
}

// Stop shuts down the listener.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.listener != nil {
		_ = m.listener.Close()
		m.listener = nil
	}
	m.mu.Unlock()
	m.wg.Wait()
}

// NotifyLocalRecordsChanged should be called after local dnsrecords are persisted (non-blocking).
func (m *Manager) NotifyLocalRecordsChanged() {
	cfg := m.dns.GetResolverSettings()
	if !cfg.ClusterEnabled || len(cfg.ClusterPeers) == 0 {
		return
	}
	if data.RecordsSourceIsReadOnly() {
		return
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		m.log.Warn("cluster: cluster_enabled but cluster_auth_token is empty; not pushing")
		return
	}
	go m.pushToAllPeers(cfg)
}

func (m *Manager) pushToAllPeers(cfg config.Config) {
	seq := m.state.NextLocalSeq()
	rec := m.dns.GetRecords()
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	nodeID := m.state.NodeID()

	payload, err := json.Marshal(RecordsFullMessage{
		Type:      TypeRecordsFull,
		NodeID:    nodeID,
		Seq:       seq,
		Timestamp: time.Now().Unix(),
		Records:   rec,
	})
	if err != nil {
		m.log.Error("cluster: marshal push", "error", err)
		return
	}

	for _, peer := range cfg.ClusterPeers {
		p := strings.TrimSpace(peer)
		if p == "" {
			continue
		}
		go m.pushOnce(p, token, payload)
	}
}

func (m *Manager) pushOnce(peerAddr, token string, payload []byte) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAddr)
	if err != nil {
		m.log.Warn("cluster: dial peer", "peer", peerAddr, "error", err)
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		m.log.Warn("cluster: write auth", "peer", peerAddr, "error", err)
		return
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		m.log.Warn("cluster: read auth reply", "peer", peerAddr, "error", err)
		return
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		m.log.Warn("cluster: auth failed", "peer", peerAddr, "body", string(frame))
		return
	}
	if err := WriteFrame(conn, payload); err != nil {
		m.log.Warn("cluster: write records", "peer", peerAddr, "error", err)
		return
	}
	m.log.Debug("cluster: pushed to peer", "peer", peerAddr)
}

// PullFromPeers requests a snapshot from each peer (heals drift). Uses first successful newer seq.
func (m *Manager) PullFromPeers(cfg config.Config) {
	if !cfg.ClusterEnabled {
		return
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		return
	}
	for _, peer := range cfg.ClusterPeers {
		p := strings.TrimSpace(peer)
		if p == "" {
			continue
		}
		m.pullOnce(p, token)
	}
}

func (m *Manager) pullOnce(peerAddr, token string) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAddr)
	if err != nil {
		return
	}
	defer conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer conn.Close()

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		return
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		return
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		return
	}
	pull, _ := json.Marshal(SimpleMessage{Type: TypePull})
	if err := WriteFrame(conn, pull); err != nil {
		return
	}
	resp, err := ReadFrame(conn)
	if err != nil {
		return
	}
	var msg RecordsFullMessage
	if err := json.Unmarshal(resp, &msg); err != nil {
		return
	}
	if msg.Type != TypeRecordsFull {
		return
	}
	records := msg.Records
	if m.state.IsDuplicate(msg.NodeID, msg.Seq) {
		return
	}
	if err := m.dns.ApplyClusterRecords(records); err != nil {
		m.log.Warn("cluster: apply pull", "error", err)
		return
	}
	m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
	m.log.Info("cluster: applied pull from peer", "peer", peerAddr, "node_id", msg.NodeID, "seq", msg.Seq)
}

func (m *Manager) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))

	cfg := m.dns.GetResolverSettings()
	token := strings.TrimSpace(cfg.ClusterAuthToken)

	frame, err := ReadFrame(conn)
	if err != nil {
		return
	}
	var auth AuthMessage
	if err := json.Unmarshal(frame, &auth); err != nil || auth.Type != TypeAuth {
		m.writeErr(conn, "auth required")
		return
	}
	if token == "" || auth.Token != token {
		fail, _ := json.Marshal(SimpleMessage{Type: TypeAuthFail, Message: "unauthorized"})
		_ = WriteFrame(conn, fail)
		return
	}
	ok, _ := json.Marshal(SimpleMessage{Type: TypeAuthOK})
	if err := WriteFrame(conn, ok); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetDeadline(time.Now().Add(120 * time.Second))
		frame, err := ReadFrame(conn)
		if err != nil {
			if err != io.EOF {
				m.log.Debug("cluster: read", "error", err)
			}
			return
		}
		var probe SimpleMessage
		if json.Unmarshal(frame, &probe) == nil && probe.Type == TypePing {
			pong, _ := json.Marshal(SimpleMessage{Type: TypePong})
			_ = WriteFrame(conn, pong)
			continue
		}
		if probe.Type == TypePull {
			m.sendCurrentSnapshot(conn)
			continue
		}
		var msg RecordsFullMessage
		if err := json.Unmarshal(frame, &msg); err != nil || msg.Type != TypeRecordsFull {
			continue
		}
		records := msg.Records
		if m.state.IsDuplicate(msg.NodeID, msg.Seq) {
			continue
		}
		if err := m.dns.ApplyClusterRecords(records); err != nil {
			m.log.Warn("cluster: apply incoming", "error", err)
			continue
		}
		m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
		m.log.Info("cluster: applied records from peer", "node_id", msg.NodeID, "seq", msg.Seq, "count", len(records))
	}
}

func (m *Manager) sendCurrentSnapshot(w io.Writer) {
	seq := m.state.CurrentLocalSeq()
	rec := m.dns.GetRecords()
	payload, err := json.Marshal(RecordsFullMessage{
		Type:      TypeRecordsFull,
		NodeID:    m.state.NodeID(),
		Seq:       seq,
		Timestamp: time.Now().Unix(),
		Records:   rec,
	})
	if err != nil {
		return
	}
	_ = WriteFrame(w, payload)
}

func (m *Manager) writeErr(conn net.Conn, msg string) {
	b, _ := json.Marshal(SimpleMessage{Type: TypeError, Message: msg})
	_ = WriteFrame(conn, b)
}
