package daemon

import (
	"sync"
	"sync/atomic"

	"github.com/chzyer/readline"
)

// ListenerSettings captures the runtime listener configuration for the daemon.
type ListenerSettings struct {
	ClientSocketPath string
	ClientTCPAddress string
	DNSPort          string
	APIPort          string
	APIEndpoint      string
	APIEnabled       bool
}

// State owns mutable runtime data for the daemon process.
type State struct {
	rlConfigMu sync.RWMutex
	rlConfig   readline.Config

	stopMu        sync.Mutex
	stopDNSCh     chan struct{}
	stoppedDNSCh  chan struct{}
	stopClosed    bool
	stoppedClosed bool

	serverStatusMu sync.RWMutex
	serverUp       bool

	daemonMode atomic.Bool

	listenerMu sync.RWMutex
	listener   ListenerSettings

	apiRunning atomic.Bool

	tuiSessionMu sync.Mutex
}

// NewState builds a State with initial runtime defaults.
func NewState() *State {
	s := &State{}
	s.ResetDNSChannels()
	return s
}

// ResetDNSChannels reinitialises the coordination channels used to control the DNS server lifecycle.
func (s *State) ResetDNSChannels() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	s.stopDNSCh = make(chan struct{})
	s.stoppedDNSCh = make(chan struct{})
	s.stopClosed = false
	s.stoppedClosed = false
}

// StopChannel returns the channel used to signal a DNS shutdown.
func (s *State) StopChannel() <-chan struct{} {
	s.stopMu.Lock()
	ch := s.stopDNSCh
	s.stopMu.Unlock()
	return ch
}

// StoppedChannel returns the channel that is closed once DNS shutdown has completed.
func (s *State) StoppedChannel() <-chan struct{} {
	s.stopMu.Lock()
	ch := s.stoppedDNSCh
	s.stopMu.Unlock()
	return ch
}

// SignalStop closes the stop channel (once) and returns the channel that should be awaited for shutdown completion.
func (s *State) SignalStop() <-chan struct{} {
	s.stopMu.Lock()
	if !s.stopClosed {
		close(s.stopDNSCh)
		s.stopClosed = true
	}
	stopped := s.stoppedDNSCh
	s.stopMu.Unlock()
	return stopped
}

// NotifyStopped closes the stopped channel (once) to indicate DNS shutdown completion.
func (s *State) NotifyStopped() {
	s.stopMu.Lock()
	if !s.stoppedClosed {
		close(s.stoppedDNSCh)
		s.stoppedClosed = true
	}
	s.stopMu.Unlock()
}

// SetServerStatus stores whether the DNS server is currently running.
func (s *State) SetServerStatus(up bool) {
	s.serverStatusMu.Lock()
	s.serverUp = up
	s.serverStatusMu.Unlock()
}

// ServerStatus reports whether the DNS server is currently running.
func (s *State) ServerStatus() bool {
	s.serverStatusMu.RLock()
	defer s.serverStatusMu.RUnlock()
	return s.serverUp
}

// SetDaemonMode toggles daemon-mode specific behaviour (e.g. query logging).
func (s *State) SetDaemonMode(enabled bool) {
	s.daemonMode.Store(enabled)
}

// DaemonMode reports whether daemon-mode behaviour is enabled.
func (s *State) DaemonMode() bool {
	return s.daemonMode.Load()
}

// SetReadlineConfig stores the readline configuration used for interactive sessions.
func (s *State) SetReadlineConfig(cfg readline.Config) {
	s.rlConfigMu.Lock()
	s.rlConfig = cfg
	s.rlConfigMu.Unlock()
}

// ReadlineConfig returns a copy of the stored readline configuration.
func (s *State) ReadlineConfig() readline.Config {
	s.rlConfigMu.RLock()
	defer s.rlConfigMu.RUnlock()
	return s.rlConfig
}

// UpdateListener applies a mutation to the stored listener settings.
func (s *State) UpdateListener(update func(*ListenerSettings)) {
	s.listenerMu.Lock()
	update(&s.listener)
	s.listenerMu.Unlock()
}

// ListenerSnapshot returns a copy of the current listener settings.
func (s *State) ListenerSnapshot() ListenerSettings {
	s.listenerMu.RLock()
	defer s.listenerMu.RUnlock()
	return s.listener
}

// SetAPIRunning records the API server running state.
func (s *State) SetAPIRunning(running bool) {
	s.apiRunning.Store(running)
}

// APIRunning reports whether the API server goroutine is currently running.
func (s *State) APIRunning() bool {
	return s.apiRunning.Load()
}

// WithTUISession synchronises access to a TUI session and executes fn while locked.
func (s *State) WithTUISession(fn func()) {
	s.tuiSessionMu.Lock()
	defer s.tuiSessionMu.Unlock()
	fn()
}

// TUISessionMutex exposes the mutex guarding interactive TUI sessions for legacy call sites.
func (s *State) TUISessionMutex() *sync.Mutex {
	return &s.tuiSessionMu
}
