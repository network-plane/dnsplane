// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"log/slog"
	"net"
	"strings"

	"dnsplane/api"
	"dnsplane/commandhandler"
	"dnsplane/daemon"
	"dnsplane/data"
)

func stopAPIAsync(state *daemon.State) {
	api.Stop(state)
}

func currentServerListeners(state *daemon.State) commandhandler.ServerListenerInfo {
	listener := state.ListenerSnapshot()
	dnsPort := strings.TrimSpace(listener.DNSPort)
	settings := data.GetInstance().GetResolverSettings()
	if dnsPort == "" {
		dnsPort = strings.TrimSpace(settings.DNSPort)
	}

	socket := strings.TrimSpace(listener.ClientSocketPath)
	tcp := strings.TrimSpace(listener.ClientTCPAddress)
	apiEndpoint := strings.TrimSpace(listener.APIEndpoint)
	if apiEndpoint == "" && settings.RESTPort != "" {
		rest := strings.TrimSpace(settings.RESTPort)
		apiBind := strings.TrimSpace(settings.APIBind)
		if apiBind != "" {
			apiEndpoint = normalizeTCPAddress(net.JoinHostPort(apiBind, rest))
		} else {
			apiEndpoint = normalizeTCPAddress(":" + rest)
		}
	}

	dnsAddr := dnsListenAddr(dnsPort)
	info := commandhandler.ServerListenerInfo{
		DNSProtocol:         "udp,tcp",
		DNSListeners:        []string{normalizeTCPAddress(dnsAddr)},
		ClientSocket:        socket,
		ClientSocketEnabled: socket != "",
		ClientTCPEndpoint:   tcp,
		ClientTCPEnabled:    tcp != "",
		ClientTCPRunning:    isClientTCPListenerRunning(),
		APIEndpoint:         apiEndpoint,
		APIEnabled:          listener.APIEnabled,
		APIRunning:          state.APIRunning(),
	}
	return info
}

func startAPIAsync(state *daemon.State, port string, log *slog.Logger) {
	if port == "" {
		port = data.GetInstance().GetResolverSettings().RESTPort
	}
	trimmed := strings.TrimSpace(port)
	st := data.GetInstance().GetResolverSettings()
	apiBind := strings.TrimSpace(st.APIBind)
	var apiEndpoint string
	if apiBind != "" {
		apiEndpoint = normalizeTCPAddress(net.JoinHostPort(apiBind, trimmed))
	} else {
		apiEndpoint = normalizeTCPAddress(":" + trimmed)
	}
	state.UpdateListener(func(info *daemon.ListenerSettings) {
		info.APIPort = trimmed
		info.APIEndpoint = apiEndpoint
		info.APIEnabled = true
	})
	if state.APIRunning() {
		return
	}
	opts := &api.ListenOptions{
		BindIP:         st.APIBind,
		TLSCertFile:    st.APITLSCertFile,
		TLSKeyFile:     st.APITLSKeyFile,
		RateLimitRPS:   st.APIRateLimitPerIP,
		RateLimitBurst: st.APIRateLimitBurst,
	}
	api.Start(state, trimmed, opts, nil, log)
}
