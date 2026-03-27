// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"dnsplane/config"
	"dnsplane/data"
)

// dashboardStatusFeature is one card in the dashboard "Protocols & features" grid (JSON for /stats/dashboard/data).
type dashboardStatusFeature struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Variant string `json:"variant"` // ok | warn | bad | neutral
}

func buildDashboardStatusFeatures(inst *data.DNSResolverData, dnsUp bool) []dashboardStatusFeature {
	cfg := inst.GetResolverSettings()
	out := make([]dashboardStatusFeature, 0, 24)

	out = append(out, dotFeature(cfg, dnsUp)...)
	out = append(out, dohFeature(cfg, dnsUp)...)
	out = append(out, dnssecValidateFeature(cfg))
	out = append(out, dnssecSignFeature(cfg))
	out = append(out, apiTLSFeature(cfg))
	out = append(out, apiAuthFeature(cfg))
	out = append(out, boolFeature("cluster", "Cluster sync", cfg.ClusterEnabled))
	out = append(out, boolFeature("full_stats", "Full stats DB", cfg.FullStats))
	out = append(out, pprofFeature(cfg))
	out = append(out, boolFeature("cache", "Resolver cache", cfg.CacheRecords))
	out = append(out, boolFeature("local_records", "Local records", cfg.LocalRecordsEnabled))
	out = append(out, boolFeature("upstream_health", "Upstream health checks", cfg.UpstreamHealthCheckEnabled))
	out = append(out, adblockFeature(inst))
	out = append(out, dnsRateLimitFeature(cfg))
	out = append(out, apiRateLimitFeature(cfg))
	out = append(out, boolFeature("stale_revalidate", "Stale-while-revalidate", cfg.StaleWhileRevalidate))
	out = append(out, cacheWarmFeature(cfg))
	out = append(out, boolFeature("refuse_any", "Refuse ANY (NOTIMP)", cfg.DNSRefuseANY))
	out = append(out, responseCapFeature(cfg))
	if cfg.DNSAmplificationMaxRatio > 0 {
		out = append(out, dashboardStatusFeature{
			Key: "amp_cap", Label: "Amplification cap", Value: "1:" + strconv.Itoa(cfg.DNSAmplificationMaxRatio), Variant: "ok",
		})
	}
	if cfg.DNSMaxEDNSUDPPayload > 0 {
		out = append(out, dashboardStatusFeature{
			Key: "edns_udp", Label: "EDNS UDP max", Value: strconv.Itoa(int(cfg.DNSMaxEDNSUDPPayload)) + " B", Variant: "ok",
		})
	}
	return out
}

func dotFeature(cfg config.Config, dnsUp bool) []dashboardStatusFeature {
	if !cfg.DOTEnabled {
		return []dashboardStatusFeature{{Key: "dot", Label: "DoT", Value: "Off", Variant: "neutral"}}
	}
	tlsOK := strings.TrimSpace(cfg.DOTCertFile) != "" && strings.TrimSpace(cfg.DOTKeyFile) != ""
	if !tlsOK {
		return []dashboardStatusFeature{{Key: "dot", Label: "DoT", Value: "On · missing TLS cert/key", Variant: "bad"}}
	}
	addr := dotListenString(cfg)
	if dnsUp {
		return []dashboardStatusFeature{{Key: "dot", Label: "DoT", Value: addr, Variant: "ok"}}
	}
	return []dashboardStatusFeature{{Key: "dot", Label: "DoT", Value: addr + " (DNS listener down)", Variant: "warn"}}
}

func dohFeature(cfg config.Config, dnsUp bool) []dashboardStatusFeature {
	if !cfg.DOHEnabled {
		return []dashboardStatusFeature{{Key: "doh", Label: "DoH", Value: "Off", Variant: "neutral"}}
	}
	tlsOK := strings.TrimSpace(cfg.DOHCertFile) != "" && strings.TrimSpace(cfg.DOHKeyFile) != ""
	if !tlsOK {
		return []dashboardStatusFeature{{Key: "doh", Label: "DoH", Value: "On · missing TLS cert/key", Variant: "bad"}}
	}
	addr := dohListenString(cfg)
	if dnsUp {
		return []dashboardStatusFeature{{Key: "doh", Label: "DoH", Value: addr, Variant: "ok"}}
	}
	return []dashboardStatusFeature{{Key: "doh", Label: "DoH", Value: addr + " (DNS listener down)", Variant: "warn"}}
}

func dnssecValidateFeature(cfg config.Config) dashboardStatusFeature {
	if !cfg.DNSSECValidate {
		return dashboardStatusFeature{Key: "dnssec_val", Label: "DNSSEC validate", Value: "Off", Variant: "neutral"}
	}
	v := "On"
	if cfg.DNSSECValidateStrict {
		v += " · strict"
	}
	if strings.TrimSpace(cfg.DNSSECTrustAnchorFile) != "" {
		v += " · anchors file"
	}
	return dashboardStatusFeature{Key: "dnssec_val", Label: "DNSSEC validate", Value: v, Variant: "ok"}
}

func dnssecSignFeature(cfg config.Config) dashboardStatusFeature {
	if !cfg.DNSSECSignEnabled {
		return dashboardStatusFeature{Key: "dnssec_sign", Label: "DNSSEC sign (local)", Value: "Off", Variant: "neutral"}
	}
	z := strings.TrimSpace(cfg.DNSSECSignZone)
	if z == "" {
		return dashboardStatusFeature{Key: "dnssec_sign", Label: "DNSSEC sign (local)", Value: "On · no zone set", Variant: "warn"}
	}
	return dashboardStatusFeature{Key: "dnssec_sign", Label: "DNSSEC sign (local)", Value: z, Variant: "ok"}
}

func apiTLSFeature(cfg config.Config) dashboardStatusFeature {
	ok := strings.TrimSpace(cfg.APITLSCertFile) != "" && strings.TrimSpace(cfg.APITLSKeyFile) != ""
	if !ok {
		return dashboardStatusFeature{Key: "api_tls", Label: "API HTTPS", Value: "Off (HTTP)", Variant: "neutral"}
	}
	return dashboardStatusFeature{Key: "api_tls", Label: "API HTTPS", Value: "On", Variant: "ok"}
}

func apiAuthFeature(cfg config.Config) dashboardStatusFeature {
	if strings.TrimSpace(cfg.APIAuthToken) == "" {
		return dashboardStatusFeature{Key: "api_auth", Label: "API auth", Value: "Open", Variant: "neutral"}
	}
	return dashboardStatusFeature{Key: "api_auth", Label: "API auth", Value: "Bearer / X-API-Token", Variant: "ok"}
}

func pprofFeature(cfg config.Config) dashboardStatusFeature {
	if !cfg.PprofEnabled {
		return dashboardStatusFeature{Key: "pprof", Label: "pprof HTTP", Value: "Off", Variant: "neutral"}
	}
	addr := strings.TrimSpace(cfg.PprofListen)
	if addr == "" {
		addr = "127.0.0.1:6060"
	}
	return dashboardStatusFeature{Key: "pprof", Label: "pprof HTTP", Value: addr, Variant: "ok"}
}

func adblockFeature(inst *data.DNSResolverData) dashboardStatusFeature {
	n := 0
	if bl := inst.GetBlockList(); bl != nil {
		n = bl.Count()
	}
	if n <= 0 {
		return dashboardStatusFeature{Key: "adblock", Label: "Adblock", Value: "Off", Variant: "neutral"}
	}
	return dashboardStatusFeature{Key: "adblock", Label: "Adblock", Value: fmt.Sprintf("%d domains", n), Variant: "ok"}
}

func dnsRateLimitFeature(cfg config.Config) dashboardStatusFeature {
	if cfg.DNSRateLimitPerIP <= 0 {
		return dashboardStatusFeature{Key: "dns_rl", Label: "DNS rate limit", Value: "Off", Variant: "neutral"}
	}
	burst := cfg.DNSRateLimitBurst
	if burst <= 0 {
		burst = 50
	}
	return dashboardStatusFeature{
		Key: "dns_rl", Label: "DNS rate limit",
		Value:   fmt.Sprintf("%.0f/s · burst %d", cfg.DNSRateLimitPerIP, burst),
		Variant: "ok",
	}
}

func apiRateLimitFeature(cfg config.Config) dashboardStatusFeature {
	if cfg.APIRateLimitPerIP <= 0 {
		return dashboardStatusFeature{Key: "api_rl", Label: "HTTP rate limit", Value: "Off", Variant: "neutral"}
	}
	burst := cfg.APIRateLimitBurst
	if burst <= 0 {
		burst = 20
	}
	return dashboardStatusFeature{
		Key: "api_rl", Label: "HTTP rate limit",
		Value:   fmt.Sprintf("%v/s · burst %d", cfg.APIRateLimitPerIP, burst),
		Variant: "ok",
	}
}

func cacheWarmFeature(cfg config.Config) dashboardStatusFeature {
	if !cfg.CacheWarmEnabled {
		return dashboardStatusFeature{Key: "cache_warm", Label: "Cache warm", Value: "Off", Variant: "neutral"}
	}
	iv := cfg.CacheWarmIntervalSeconds
	if iv <= 0 {
		iv = 10
	}
	return dashboardStatusFeature{Key: "cache_warm", Label: "Cache warm", Value: fmt.Sprintf("On · every %ds", iv), Variant: "ok"}
}

func responseCapFeature(cfg config.Config) dashboardStatusFeature {
	mode := strings.ToLower(strings.TrimSpace(cfg.DNSResponseLimitMode))
	if mode == "" {
		mode = "sliding_window"
	}
	if mode == "rrl" {
		if cfg.DNSRRLMaxPerBucket <= 0 {
			return dashboardStatusFeature{Key: "resp_cap", Label: "Response cap", Value: "RRL · off (no bucket)", Variant: "neutral"}
		}
		win := cfg.DNSRRLWindowSeconds
		if win <= 0 {
			win = 1
		}
		return dashboardStatusFeature{
			Key: "resp_cap", Label: "Response cap",
			Value:   fmt.Sprintf("RRL · %d / %ds", cfg.DNSRRLMaxPerBucket, win),
			Variant: "ok",
		}
	}
	if cfg.DNSMaxResponsesPerIPWindow <= 0 {
		return dashboardStatusFeature{Key: "resp_cap", Label: "Response cap", Value: "Off", Variant: "neutral"}
	}
	wsec := cfg.DNSSlidingWindowSeconds
	if wsec <= 0 {
		wsec = 1
	}
	return dashboardStatusFeature{
		Key: "resp_cap", Label: "Response cap",
		Value:   fmt.Sprintf("Sliding · %d / %ds", cfg.DNSMaxResponsesPerIPWindow, wsec),
		Variant: "ok",
	}
}

func boolFeature(key, label string, on bool) dashboardStatusFeature {
	if on {
		return dashboardStatusFeature{Key: key, Label: label, Value: "On", Variant: "ok"}
	}
	return dashboardStatusFeature{Key: key, Label: label, Value: "Off", Variant: "neutral"}
}

func dotListenString(c config.Config) string {
	port := strings.TrimSpace(c.DOTPort)
	if port == "" {
		port = "853"
	}
	bind := strings.TrimSpace(c.DOTBind)
	if bind == "" {
		return ":" + port
	}
	return net.JoinHostPort(bind, port)
}

func dohListenString(c config.Config) string {
	port := strings.TrimSpace(c.DOHPort)
	if port == "" {
		port = "8443"
	}
	bind := strings.TrimSpace(c.DOHBind)
	addr := ":" + port
	if bind != "" {
		addr = net.JoinHostPort(bind, port)
	}
	path := strings.TrimSpace(c.DOHPath)
	if path == "" {
		path = "/dns-query"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return addr + path
}
