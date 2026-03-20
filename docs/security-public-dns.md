# Operating dnsplane as a public DNS resolver

This document summarizes safety-related configuration for running dnsplane exposed to untrusted clients on the internet.

## Network

- Prefer **firewall allowlists** so only intended clients reach UDP/TCP 53 (and DoT/DoH ports if enabled).
- Bind sensitive listeners to loopback or a management interface: `dns_bind`, `api_bind`.
- The REST API should not be exposed without **TLS** (`api_tls_cert` / `api_tls_key`) and **`api_auth_token`**.

## Rate limiting and abuse

- **Per-query rate limit:** `dns_rate_limit_rps` / `dns_rate_limit_burst` (token bucket per client IP).
- **Response-side limits** (`dns_response_limit_mode`):
  - **`sliding_window`** (default mode): caps **responses per client IP** per time window (`dns_sliding_window_seconds`, `dns_max_responses_per_ip_window`). Disabled when max responses is 0.
  - **`rrl`**: approximate per-(client IP, QNAME) limiting (`dns_rrl_max_per_bucket`, `dns_rrl_window_seconds`, `dns_rrl_slip`). Use when you need BIND-style behavior; tuning is more involved.

Metrics: `dnsplane_dns_limiter_drops_total{reason=...}` on `/metrics`.

## Inbound DoT and DoH

- **DoT:** `dot_enabled`, `dot_bind`, `dot_port` (default 853), `dot_cert_file`, `dot_key_file`. Uses a separate **tcp-tls** listener from plain DNS.
- **DoH:** `doh_enabled`, `doh_bind`, `doh_port` (default 8443), `doh_path` (default `/dns-query`), `doh_cert_file`, `doh_key_file`. Serves RFC 8484 `application/dns-message` over **HTTPS** only.

Rotate TLS certificates before expiry; use short-lived certs (e.g. ACME) where possible.

## DNSSEC validation (best effort)

- `dnssec_validate`: when enabled, dnsplane verifies **RRSIG** records **when DNSKEY material is present in the upstream response**. It does **not** perform full chain validation from the DNS root (no iterative DS/DNSKEY chase in this version).
- `dnssec_validate_strict`: if verification fails (bogus signatures), return **SERVFAIL** instead of passing the answer.
- The **AD** bit is set on responses only when validation succeeded **and** the client sent **DNSSEC OK (DO)** in EDNS0.
- `dnssec_trust_anchor_file` is reserved for future root/anchor handling.

Metrics: `dnsplane_dnssec_outcomes_total{outcome=...}`.

## DNSSEC signing (local authoritative data)

For answers served from **local records** (`dnsrecords.json` / API), dnsplane can attach **RRSIG** when the client sends **DNSSEC OK (DO)** in EDNS0:

- **`dnssec_sign_enabled`:** enable signing (requires zone and key paths).
- **`dnssec_sign_zone`:** zone apex FQDN (e.g. `example.com.`).
- **`dnssec_sign_key_file`:** path to the public **DNSKEY** file (BIND `dnssec-keygen` produces `K*.<zone>.*.key`).
- **`dnssec_sign_private_key_file`:** path to the matching **private** file (`K*.<zone>.*.private`).

Signing is **on-the-fly** (not pre-signed static zones). The **AD** bit is set on signed local answers when DO is set. Generate keys with BIND `dnssec-keygen` (or compatible tools); **restart the dnsplane process** after changing keys or these settings so the resolver reloads the signer.

## Other hardening

- **`dns_refuse_any`:** return NOTIMP for `ANY` queries (reduces amplification and scanner noise).
- **`dns_max_edns_udp_payload`:** cap EDNS UDP payload size on responses (e.g. 1232).
- **`dns_amplification_max_ratio`:** cap packed response size vs packed request.

## Threat model

dnsplane is a forwarding resolver with local records. For high-risk deployments, combine application limits with **network-level** DDoS protection and monitoring.
