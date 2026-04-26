// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

// Package cluster implements multi-node DNS record sync over TCP (length-prefixed JSON).
// Cache and per-node state are not replicated. Use a single writer or accept LWW by sequence.
package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	peers      *peerTracker
	lookupSRV  SRVLookup // nil uses net.LookupSRV

	peerMu           sync.RWMutex
	effectivePeers   []string
	lastDiscSRV      string
	lastDiscErr      string
	lastDiscAt       time.Time
	srvResolvedCount int

	mu       sync.Mutex
	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// SetSRVLookup replaces DNS SRV resolution (tests).
func (m *Manager) SetSRVLookup(fn SRVLookup) {
	m.lookupSRV = fn
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
		peers:      newPeerTracker(),
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
	if NormalizeSyncPolicy(cfg.ClusterSyncPolicy) == SyncPolicyPrimaryWriter && len(cfg.ClusterAllowedWriterNodeIDs) == 0 {
		m.log.Warn("cluster: cluster_sync_policy=primary_writer but cluster_allowed_writer_node_ids is empty; incoming snapshots will be rejected")
	}
	m.refreshEffectivePeers()
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
					m.PullFromPeers(m.dns.GetResolverSettings())
				}
			}
		}()
	}

	discInt := cfg.ClusterDiscoveryIntervalSeconds
	if strings.TrimSpace(cfg.ClusterDiscoverySRV) != "" && discInt > 0 {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			t := time.NewTicker(time.Duration(discInt) * time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx2.Done():
					return
				case <-t.C:
					m.refreshEffectivePeers()
				}
			}
		}()
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx2.Done():
				return
			case <-t.C:
				m.probeAllPeers()
			}
		}
	}()

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

// StatusSnapshot returns runtime cluster status for dashboard / TUI.
func (m *Manager) StatusSnapshot() StatusSnapshot {
	cfg := m.dns.GetResolverSettings()
	if !cfg.ClusterEnabled {
		return StatusSnapshot{Enabled: false}
	}
	listen := strings.TrimSpace(cfg.ClusterListenAddr)
	if listen == "" {
		listen = ":7946"
	}
	adv := strings.TrimSpace(cfg.ClusterAdvertiseAddr)
	if adv == "" {
		adv = ClusterDialAddress(listen, "")
	}
	peerAddrs := m.listPeerTargets(cfg)
	gts, gn := m.state.GlobalLWWLast()
	m.peerMu.RLock()
	discErr := m.lastDiscErr
	discAt := m.lastDiscAt
	discSRV := m.lastDiscSRV
	srvN := m.srvResolvedCount
	m.peerMu.RUnlock()
	allow := append([]string(nil), cfg.ClusterAllowedWriterNodeIDs...)
	snap := StatusSnapshot{
		Enabled:                     true,
		NodeID:                      m.state.NodeID(),
		ListenAddr:                  listen,
		AdvertiseAddr:               adv,
		ReplicaOnly:                 cfg.ClusterReplicaOnly,
		ClusterAdmin:                cfg.ClusterAdmin,
		Peers:                       m.peers.snapshot(peerAddrs),
		LocalSeq:                    m.state.CurrentLocalSeq(),
		ClusterPortGuess:            adv,
		SyncPolicy:                  NormalizeSyncPolicy(cfg.ClusterSyncPolicy),
		AllowedWriterNodeIDs:        allow,
		LastGlobalLWWUnix:           gts,
		LastGlobalLWWNodeID:         gn,
		ClusterDiscoverySRV:         strings.TrimSpace(cfg.ClusterDiscoverySRV),
		ClusterDiscoveryIntervalSec: cfg.ClusterDiscoveryIntervalSeconds,
		DiscoveryLastError:          discErr,
		StaticPeerCount:             countNonEmptyStrings(cfg.ClusterPeers),
		SRVPeerCount:                srvN,
		EffectivePeerCount:          len(peerAddrs),
	}
	if discSRV != "" && !discAt.IsZero() {
		snap.DiscoveryLastRefresh = discAt.UTC().Format(time.RFC3339)
	}
	return snap
}

// JoinInfo returns material for operators to register this node on a full server.
func (m *Manager) JoinInfo() (nodeID, listenAddr, dialAddr, tokenFingerprintSHA256 string) {
	cfg := m.dns.GetResolverSettings()
	nodeID = m.state.NodeID()
	listenAddr = strings.TrimSpace(cfg.ClusterListenAddr)
	if listenAddr == "" {
		listenAddr = ":7946"
	}
	dialAddr = strings.TrimSpace(cfg.ClusterAdvertiseAddr)
	if dialAddr == "" {
		dialAddr = ClusterDialAddress(listenAddr, "")
	}
	tok := strings.TrimSpace(cfg.ClusterAuthToken)
	if tok != "" {
		sum := sha256.Sum256([]byte(tok))
		tokenFingerprintSHA256 = hex.EncodeToString(sum[:])
	}
	return nodeID, listenAddr, dialAddr, tokenFingerprintSHA256
}

// ForcePull runs PullFromPeers with current settings.
func (m *Manager) ForcePull() {
	m.PullFromPeers(m.dns.GetResolverSettings())
}

// PushRecordsToPeer sends a full snapshot to one peer (uses next local sequence).
func (m *Manager) PushRecordsToPeer(peerAddr string) error {
	cfg := m.dns.GetResolverSettings()
	if !cfg.ClusterEnabled {
		return fmt.Errorf("cluster: not enabled")
	}
	if cfg.ClusterReplicaOnly {
		return fmt.Errorf("cluster: replica-only node does not push")
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		return fmt.Errorf("cluster: cluster_auth_token empty")
	}
	seq := m.state.NextLocalSeq()
	rec := m.dns.GetRecords()
	payload, err := json.Marshal(RecordsFullMessage{
		Type:      TypeRecordsFull,
		NodeID:    m.state.NodeID(),
		Seq:       seq,
		Timestamp: time.Now().Unix(),
		Records:   rec,
	})
	if err != nil {
		return err
	}
	p := strings.TrimSpace(peerAddr)
	if p == "" {
		return fmt.Errorf("cluster: empty peer address")
	}
	m.pushOnce(p, token, payload)
	return nil
}

// AdminPushConfig sends admin_config_apply to a peer. Requires cluster_admin on this node.
func (m *Manager) AdminPushConfig(peerAddr string, apply AdminConfigApplyMessage) error {
	cfg := m.dns.GetResolverSettings()
	if !cfg.ClusterAdmin {
		return fmt.Errorf("cluster: cluster_admin is false on this node")
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		return fmt.Errorf("cluster: cluster_auth_token empty")
	}
	adminTok := strings.TrimSpace(cfg.ClusterAdminToken)
	if adminTok == "" {
		return fmt.Errorf("cluster: set cluster_admin_token on this node to send admin")
	}
	apply.Type = TypeAdminConfigApply
	apply.AdminToken = adminTok
	payload, err := json.Marshal(apply) // #nosec G117 -- intentional authenticated cluster admin payload
	if err != nil {
		return err
	}
	p := strings.TrimSpace(peerAddr)
	if p == "" {
		return fmt.Errorf("cluster: empty peer address")
	}
	d := net.Dialer{Timeout: 8 * time.Second}
	conn, err := d.Dial("tcp", p)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(45 * time.Second))

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		return err
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		return err
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		return fmt.Errorf("cluster: auth failed: %s", string(frame))
	}
	if err := WriteFrame(conn, payload); err != nil {
		return err
	}
	resp, err := ReadFrame(conn)
	if err != nil {
		return err
	}
	var ack SimpleMessage
	if err := json.Unmarshal(resp, &ack); err != nil {
		return err
	}
	if ack.Type == TypeAdminConfigFail {
		return fmt.Errorf("cluster: remote: %s", ack.Message)
	}
	if ack.Type != TypeAdminConfigOK {
		return fmt.Errorf("cluster: unexpected response: %s", string(resp))
	}
	return nil
}

func (m *Manager) probeAllPeers() {
	cfg := m.dns.GetResolverSettings()
	if !cfg.ClusterEnabled {
		return
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		return
	}
	for _, p := range m.listPeerTargets(cfg) {
		m.probeOnce(p, token)
	}
}

func (m *Manager) probeOnce(peerAddr, token string) {
	start := time.Now()
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAddr)
	if err != nil {
		m.peers.recordProbe(peerAddr, false, 0, err.Error())
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		m.peers.recordProbe(peerAddr, false, 0, err.Error())
		return
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		m.peers.recordProbe(peerAddr, false, 0, err.Error())
		return
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		m.peers.recordProbe(peerAddr, false, 0, "auth failed")
		return
	}
	ping, _ := json.Marshal(SimpleMessage{Type: TypePing})
	if err := WriteFrame(conn, ping); err != nil {
		m.peers.recordProbe(peerAddr, false, 0, err.Error())
		return
	}
	if _, err := ReadFrame(conn); err != nil {
		m.peers.recordProbe(peerAddr, false, 0, err.Error())
		return
	}
	rttMs := float64(time.Since(start).Milliseconds())
	m.peers.recordProbe(peerAddr, true, rttMs, "")
}

// NotifyLocalRecordsChanged triggers a non-blocking push to peers after local dnsrecords are saved.
func (m *Manager) NotifyLocalRecordsChanged() {
	cfg := m.dns.GetResolverSettings()
	targets := m.listPeerTargets(cfg)
	if !cfg.ClusterEnabled || len(targets) == 0 {
		return
	}
	if cfg.ClusterReplicaOnly {
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
	targets := m.listPeerTargets(cfg)
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

	for _, p := range targets {
		go m.pushOnce(p, token, payload)
	}
}

func (m *Manager) pushOnce(peerAddr, token string, payload []byte) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAddr)
	if err != nil {
		m.peers.recordOutbound(peerAddr, false, err.Error())
		m.log.Warn("cluster: dial peer", "peer", peerAddr, "error", err)
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		m.peers.recordOutbound(peerAddr, false, err.Error())
		m.log.Warn("cluster: write auth", "peer", peerAddr, "error", err)
		return
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		m.peers.recordOutbound(peerAddr, false, err.Error())
		m.log.Warn("cluster: read auth reply", "peer", peerAddr, "error", err)
		return
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		m.peers.recordOutbound(peerAddr, false, "auth failed")
		m.log.Warn("cluster: auth failed", "peer", peerAddr, "body", string(frame))
		return
	}
	if err := WriteFrame(conn, payload); err != nil {
		m.peers.recordOutbound(peerAddr, false, err.Error())
		m.log.Warn("cluster: write records", "peer", peerAddr, "error", err)
		return
	}
	m.peers.recordOutbound(peerAddr, true, "")
	m.log.Debug("cluster: pushed to peer", "peer", peerAddr)
}

// PullFromPeers requests a snapshot from each peer (heals drift).
func (m *Manager) PullFromPeers(cfg config.Config) {
	if !cfg.ClusterEnabled {
		return
	}
	token := strings.TrimSpace(cfg.ClusterAuthToken)
	if token == "" {
		return
	}
	for _, p := range m.listPeerTargets(cfg) {
		m.pullOnce(p, token)
	}
}

func (m *Manager) pullOnce(peerAddr, token string) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAddr)
	if err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	auth, _ := json.Marshal(AuthMessage{Type: TypeAuth, Token: token})
	if err := WriteFrame(conn, auth); err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	frame, err := ReadFrame(conn)
	if err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	var ok SimpleMessage
	if err := json.Unmarshal(frame, &ok); err != nil || ok.Type != TypeAuthOK {
		m.peers.recordPull(peerAddr, false)
		return
	}
	pull, _ := json.Marshal(SimpleMessage{Type: TypePull})
	if err := WriteFrame(conn, pull); err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	resp, err := ReadFrame(conn)
	if err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	var msg RecordsFullMessage
	if err := json.Unmarshal(resp, &msg); err != nil {
		m.peers.recordPull(peerAddr, false)
		return
	}
	if msg.Type != TypeRecordsFull {
		m.peers.recordPull(peerAddr, false)
		return
	}
	if m.state.IsDuplicate(msg.NodeID, msg.Seq) {
		m.peers.recordPull(peerAddr, true)
		return
	}
	cfgNow := m.dns.GetResolverSettings()
	applySnap, snapReason := ShouldApplySnapshot(cfgNow, m.state, &msg)
	if !applySnap {
		if m.log != nil && snapReason != "" {
			m.log.Debug("cluster: skip pull apply", "reason", snapReason, "peer", peerAddr, "node_id", msg.NodeID)
		}
		m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
		m.peers.recordPull(peerAddr, true)
		return
	}
	records := msg.Records
	if err := m.dns.ApplyClusterRecords(records); err != nil {
		m.log.Warn("cluster: apply pull", "error", err)
		m.peers.recordPull(peerAddr, false)
		return
	}
	m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
	if NormalizeSyncPolicy(cfgNow.ClusterSyncPolicy) == SyncPolicyGlobalLWW {
		ts := msg.Timestamp
		if ts <= 0 {
			ts = 1
		}
		m.state.commitGlobalLWW(ts, msg.NodeID)
	}
	m.peers.recordPull(peerAddr, true)
	m.log.Info("cluster: applied pull from peer", "peer", peerAddr, "node_id", msg.NodeID, "seq", msg.Seq)
}

func (m *Manager) serveConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	remote := conn.RemoteAddr().String()
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
		var adminMsg AdminConfigApplyMessage
		if json.Unmarshal(frame, &adminMsg) == nil && adminMsg.Type == TypeAdminConfigApply {
			if err := m.applyAdminConfigFromMessage(&adminMsg); err != nil {
				fail, _ := json.Marshal(SimpleMessage{Type: TypeAdminConfigFail, Message: err.Error()})
				_ = WriteFrame(conn, fail)
				continue
			}
			ack, _ := json.Marshal(SimpleMessage{Type: TypeAdminConfigOK})
			_ = WriteFrame(conn, ack)
			continue
		}
		var msg RecordsFullMessage
		if err := json.Unmarshal(frame, &msg); err != nil || msg.Type != TypeRecordsFull {
			continue
		}
		m.peers.recordInbound(remote)
		records := msg.Records
		if m.state.IsDuplicate(msg.NodeID, msg.Seq) {
			continue
		}
		cfgNow := m.dns.GetResolverSettings()
		applyOK, reason := ShouldApplySnapshot(cfgNow, m.state, &msg)
		if !applyOK {
			if m.log != nil && reason != "" {
				m.log.Debug("cluster: skip inbound apply", "reason", reason, "node_id", msg.NodeID)
			}
			m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
			continue
		}
		if err := m.dns.ApplyClusterRecords(records); err != nil {
			m.log.Warn("cluster: apply incoming", "error", err)
			continue
		}
		m.state.CommitPeerSeq(msg.NodeID, msg.Seq)
		if NormalizeSyncPolicy(cfgNow.ClusterSyncPolicy) == SyncPolicyGlobalLWW {
			ts := msg.Timestamp
			if ts <= 0 {
				ts = 1
			}
			m.state.commitGlobalLWW(ts, msg.NodeID)
		}
		m.log.Info("cluster: applied records from peer", "node_id", msg.NodeID, "seq", msg.Seq, "count", len(records))
	}
}

func (m *Manager) applyAdminConfigFromMessage(msg *AdminConfigApplyMessage) error {
	cfg := m.dns.GetResolverSettings()
	if strings.TrimSpace(cfg.ClusterAdminToken) == "" {
		return fmt.Errorf("remote admin disabled (empty cluster_admin_token)")
	}
	if msg.AdminToken != cfg.ClusterAdminToken {
		return fmt.Errorf("invalid admin token")
	}
	s := cfg
	if msg.AuthToken != "" {
		s.ClusterAuthToken = msg.AuthToken
	}
	if msg.Peers != nil {
		s.ClusterPeers = append([]string(nil), msg.Peers...)
	}
	if msg.ReplicaOnly != nil {
		s.ClusterReplicaOnly = *msg.ReplicaOnly
	}
	if msg.RejectLocal != nil {
		s.ClusterRejectLocalWrites = *msg.RejectLocal
	}
	if msg.AdminLocal != nil {
		s.ClusterAdmin = *msg.AdminLocal
	}
	if msg.SyncInterval != nil {
		s.ClusterSyncIntervalSeconds = *msg.SyncInterval
	}
	m.dns.UpdateSettings(s)
	m.refreshEffectivePeers()
	return nil
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

func (m *Manager) refreshEffectivePeers() {
	cfg := m.dns.GetResolverSettings()
	static := cfg.ClusterPeers
	srvName := strings.TrimSpace(cfg.ClusterDiscoverySRV)
	var srvTargets []string
	var err error
	if srvName != "" {
		lookup := m.lookupSRV
		if lookup == nil {
			lookup = net.LookupSRV
		}
		srvTargets, err = LookupSRVTargets(lookup, srvName)
		if err != nil && m.log != nil {
			m.log.Warn("cluster: SRV lookup", "srv", srvName, "error", err)
		}
	}
	merged := MergePeerAddrs(static, srvTargets)
	m.peerMu.Lock()
	m.effectivePeers = merged
	m.lastDiscSRV = srvName
	if err != nil {
		m.lastDiscErr = err.Error()
	} else {
		m.lastDiscErr = ""
	}
	m.lastDiscAt = time.Now()
	m.srvResolvedCount = len(srvTargets)
	m.peerMu.Unlock()
}

func (m *Manager) listPeerTargets(cfg config.Config) []string {
	m.peerMu.RLock()
	out := m.effectivePeers
	m.peerMu.RUnlock()
	if len(out) > 0 {
		cp := make([]string, len(out))
		copy(cp, out)
		return cp
	}
	var fallback []string
	for _, p := range cfg.ClusterPeers {
		if s := strings.TrimSpace(p); s != "" {
			fallback = append(fallback, s)
		}
	}
	return fallback
}

func countNonEmptyStrings(ss []string) int {
	n := 0
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}
