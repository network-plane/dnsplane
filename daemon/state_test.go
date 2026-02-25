// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package daemon

import (
	"testing"
)

func TestNewState(t *testing.T) {
	s := NewState()
	if s == nil {
		t.Fatal("NewState returned nil")
	}
	if s.ServerStatus() {
		t.Error("new state should have ServerStatus false")
	}
	if s.APIRunning() {
		t.Error("new state should have APIRunning false")
	}
}

func TestState_ServerStatus(t *testing.T) {
	s := NewState()
	s.SetServerStatus(true)
	if !s.ServerStatus() {
		t.Error("ServerStatus() = false after SetServerStatus(true)")
	}
	s.SetServerStatus(false)
	if s.ServerStatus() {
		t.Error("ServerStatus() = true after SetServerStatus(false)")
	}
}

func TestState_APIRunning(t *testing.T) {
	s := NewState()
	s.SetAPIRunning(true)
	if !s.APIRunning() {
		t.Error("APIRunning() = false after SetAPIRunning(true)")
	}
	s.SetAPIRunning(false)
	if s.APIRunning() {
		t.Error("APIRunning() = true after SetAPIRunning(false)")
	}
}

func TestState_ListenerSnapshot(t *testing.T) {
	s := NewState()
	snapshot := s.ListenerSnapshot()
	if snapshot.DNSPort != "" || snapshot.APIPort != "" {
		t.Errorf("new listener snapshot should have zero values, got %+v", snapshot)
	}
	s.UpdateListener(func(l *ListenerSettings) {
		l.DNSPort = "53"
		l.APIPort = "8080"
	})
	snapshot = s.ListenerSnapshot()
	if snapshot.DNSPort != "53" || snapshot.APIPort != "8080" {
		t.Errorf("ListenerSnapshot = %+v", snapshot)
	}
}

func TestState_SignalStop_NotifyStopped(t *testing.T) {
	s := NewState()
	stopped := s.SignalStop()
	select {
	case <-stopped:
		t.Fatal("StoppedChannel should not be closed until NotifyStopped")
	default:
	}
	s.NotifyStopped()
	select {
	case <-stopped:
	default:
		t.Error("StoppedChannel should be closed after NotifyStopped")
	}
}
