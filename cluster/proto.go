// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"encoding/binary"
	"fmt"
	"io"

	"dnsplane/dnsrecords"
	"dnsplane/safecast"
)

// MaxFrameBytes caps a single JSON frame (records snapshot).
const MaxFrameBytes = 64 << 20 // 64 MiB

// WriteFrame writes a length-prefixed JSON payload (big-endian uint32 length).
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxFrameBytes {
		return fmt.Errorf("cluster: frame %d bytes exceeds max %d", len(payload), MaxFrameBytes)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], safecast.IntToUint32Clamp(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// ReadFrame reads one length-prefixed frame.
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxFrameBytes {
		return nil, fmt.Errorf("cluster: frame length %d exceeds max %d", n, MaxFrameBytes)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// Message types (JSON "type" field).
const (
	TypeAuth             = "auth"
	TypeAuthOK           = "auth_ok"
	TypeAuthFail         = "auth_fail"
	TypeRecordsFull      = "records_full"
	TypePull             = "pull"
	TypePing             = "ping"
	TypePong             = "pong"
	TypeError            = "error"
	TypeAdminConfigApply = "admin_config_apply"
	TypeAdminConfigOK    = "admin_config_ok"
	TypeAdminConfigFail  = "admin_config_fail"
)

// AuthMessage is the first message from a client.
type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// RecordsFullMessage carries a full dnsrecords snapshot.
type RecordsFullMessage struct {
	Type      string                 `json:"type"`
	NodeID    string                 `json:"node_id"`
	Seq       uint64                 `json:"seq"`
	Timestamp int64                  `json:"timestamp_unix,omitempty"`
	Records   []dnsrecords.DNSRecord `json:"records"`
}

// SimpleMessage is ping/pong/auth_ok/auth_fail/error.
type SimpleMessage struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

// AdminConfigApplyMessage updates cluster-related fields on a peer (requires matching admin_token on target).
type AdminConfigApplyMessage struct {
	Type         string   `json:"type"`
	AdminToken   string   `json:"admin_token"`
	AuthToken    string   `json:"cluster_auth_token,omitempty"`
	Peers        []string `json:"cluster_peers,omitempty"`
	ReplicaOnly  *bool    `json:"cluster_replica_only,omitempty"`
	RejectLocal  *bool    `json:"cluster_reject_local_writes,omitempty"`
	AdminLocal   *bool    `json:"cluster_admin,omitempty"`
	SyncInterval *int     `json:"cluster_sync_interval_seconds,omitempty"`
}
