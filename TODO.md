# dnsplane TODO

---

## 1. Security

- [x] API auth: config `api_auth_token`; middleware in api/server.go; exempt GET/HEAD `/health` and `/ready`.
- [x] API TLS: `api_tls_cert` + `api_tls_key` → `ListenAndServeTLS` (ACME left for future automation).
- [x] rate limiting: `api_rate_limit_rps` / `api_rate_limit_burst`, `dns_rate_limit_rps` / `dns_rate_limit_burst` (per-IP token bucket).
- [x] DoT/DoH upstream: per-server `transport` (`udp`|`tcp`|`dot`|`doh`), `doh_url` for DoH; resolver uses miekg `tcp-tls` / `https`.
- [x] DNS over TCP: TCP listener alongside UDP; same handler (`main.go`).
- [x] Amplification mitigation: `dns_amplification_max_ratio` (packed response vs request; 0 = off).
- [x] Configurable bind: `dns_bind`, `api_bind` (empty = all interfaces).

---

## 2. DoT/DoH server and DNSSEC

_Inbound_ listeners on dnsplane. For **querying upstreams** over TLS/HTTPS see §1 (DoT/DoH upstream)._

- [x] DoT server: `dot_enabled`, `dot_bind`, `dot_port`, `dot_cert_file`, `dot_key_file`; tcp-tls listener; shared `dnsserve.ServeDNS` path.
- [x] DoH server: `doh_*`, RFC 8484 `application/dns-message`, TLS; `main_dns_inbound.go`.
- [x] DNSSEC validation (best-effort): `dnssec_validate`, `dnssec_validate_strict`; RRSIG verify when DNSKEY in message; AD with DO; metrics.
- [x] Response-side limits: `dns_response_limit_mode` `sliding_window` (default) vs `rrl`; Prometheus drops; see [docs/security-public-dns.md](docs/security-public-dns.md).
- [x] DNSSEC signing (optional): sign local zone (dnsrecords.json); BIND `K*.key`/`K*.private`; RRSIG on-the-fly when client sets DO (`dnssec_sign_*`); restart process to load keys.

---

## 3. Package building (RPM, DEB)

- [ ] Add spec file (e.g. packaging/dnsplane.spec): build binary, install to /usr/bin or /usr/local/dnsplane, install systemd unit, optional conffiles and dnsplane user. Version/Release from tag or VERSION file.
- [ ] Add debian/: control, rules, changelog, compat, optional dnsplane.install; rules builds binary, installs binary and systemd unit.
- [ ] Document in README or docs/packaging.md: build and sign with **pbuild** only; no GitHub Actions/cloud CI for building/signing (private keys must not be in GitHub).
- [ ] Optional: APK (Alpine), Homebrew formula (macOS), Windows MSI/zip.

---

## 4. Various

- [ ] `GET /dns/records`: optional query params `?name=`, `?type=` for filtering; mirror `dnsrecords.List`.
- [ ] Per-server fallback: optional second address when whitelist server fails (resolver + dnsservers).
- [ ] Record PATCH: use identifier (e.g. name+type+value) for update/delete to avoid overwrites.
- [ ] Extra micro-opts if profiling shows a clear target.

---

## 5. Clustered DNS (multi-node sync)

- [ ] Topology beyond current LWW: explicit primary–replica mode, Raft/leader election, or stronger conflict policies (configurable).
- [ ] Discovery: optional **DNS SRV** (or similar) for dynamic peer discovery (today: static `cluster_peers` only).

---

## 6. ISPConfig and cPanel compatibility

Goal: replace BIND/PowerDNS behind ISPConfig or cPanel; panels keep managing zones (DB → zone files → reload), dnsplane serves from those files.

- [ ] BIND zone file parser: parse $ORIGIN, $TTL, SOA, NS, A, AAAA, CNAME, MX, TXT, PTR and other common RR types; relative names and multi-line records; output loadable into dnsplane record store.
- [ ] Multi-zone / zone-aware loading: merge all zones into one store with FQDN keys, or per-zone store with longest-match lookup; correct NS/SOA per zone.
- [ ] Zone file source config: records_source type "bind_zones" with location (directory + optional named.conf for zone→file mapping); on startup and reload, scan, parse, load into resolver.
- [ ] Reload trigger: file watch (inotify/fsnotify) on zone directory; and/or API (e.g. POST /reload) for panel script to call instead of rndc; optionally rndc-compatible daemon (port 953) so rndc reload works unchanged.
- [ ] Zone transfer: if secondaries pull from this server, add AXFR (and optionally IXFR) so dnsplane can act as primary.
- [ ] ISPConfig docs: path where ISPConfig writes zones and named.conf.local (e.g. /etc/bind); records_source + file watch or reload-API script.
- [ ] cPanel docs: zone path (e.g. /var/named/), reload flow (rndc or script); point dnsplane at path + watch or wrapper script; note WHM Nameserver selection and PowerDNS variant.
- [ ] Optional: RFC 2136 dynamic updates (nsupdate) for panels/users that use them.
- [ ] README: add "Using dnsplane with ISPConfig" and "Using dnsplane with cPanel" (config example, reload method, wrapper scripts).
