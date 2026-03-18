# dnsplane TODO

---

## 1. Testing and CI

- [x] Logger tests: unit tests for pure helpers in logger/logger.go (severity, path); mock file I/O.
- [x] Commandhandler tests: parsing/validation for TUI commands (server add/update, record add/remove); mirror dnsservers apply_test style.
- [x] Integration test: add one more resolver test (e.g. no local/cache/upstream → NXDOMAIN or empty).
- [x] CI: workflow .github/workflows/test.yml — `make test`, `make vet` on push/PR. Tests/vet only; no package build/sign in GitHub (use pbuild).

---

## 2. Documentation and dev

- [x] .gitignore: narrow `*.json` to specific names or use examples/ so example configs can be committed.
- [x] Add example config (e.g. examples/dnsplane.json or dnsplane-example.json); ensure not ignored.
- [x] CONTRIBUTING: add line that new code should include tests; `make test` and `make fuzz` expected.
- [x] README: mention AAAA and PTR in resolution; document DoT/DoH/DNSSEC when implemented.

---

## 3. REST API (data only — no config endpoints)

No endpoints for dnsplane.json or any config.

- [x] **Records:** Add `PUT /dns/records` (or PATCH): update by name+type+value. Wire to dnsrecords + data.UpdateRecords.
- [x] **Records:** Add `DELETE /dns/records`: identify by name, type, value (query or body). Wire to dnsrecords remove + data.UpdateRecords.
- [x] **DNS servers:** Add `POST /dns/servers`: body address, port, active, local_resolver, adblocker, domain_whitelist. Validate via dnsservers; persist via data.SaveDNSServers.
- [x] **DNS servers:** Add `PUT /dns/servers/{address}`: update existing server.
- [x] **DNS servers:** Add `DELETE /dns/servers/{address}`.
- [x] **Adblock:** Add `GET /adblock/domains` (optional: `GET /adblock/sources`).
- [x] **Adblock:** Add `POST /adblock/domains`: body single domain or list.
- [x] **Adblock:** Add `DELETE /adblock/domains`: body or query domain/list.
- [x] **Adblock:** Add `POST /adblock/clear` or `DELETE /adblock/domains` for all.
- [x] **Cache:** Add `GET /cache` (read-only).
- [x] **Cache:** Add `POST /cache/clear` or `DELETE /cache`.

---

## 4. DNS server health checks (upstream)

- [x] Config: `upstream_health_check_enabled`, `upstream_health_check_failures`, `upstream_health_check_interval_seconds`, `upstream_health_check_query_name` (see [docs/upstream-health.md](docs/upstream-health.md)).
- [x] Proactive probes: periodic **A** query per active upstream; mark down after N consecutive failures; exclude from forwarding; recover on probe success or successful forward.
- [x] Logging: warn when an upstream becomes unhealthy.
- [x] API: `GET /dns/servers` includes `upstream_health`; `GET /dns/upstreams/health`; curl examples in README and docs.

---

## 5. Observability

- [ ] /metrics: Prometheus-style metrics (histograms/summaries for DNS latency; optional label by query type). _Note: in-app `/stats/perf` already has latency histograms and `by_query_type`; /metrics is for external scraping._
- [ ] Structured logging: add fields (query name, type, upstream, duration) in DNS layer.
- [ ] Expose build/version in /stats and optionally /ready or /version (Go version, OS, arch).

---

## 6. Security

- [ ] Optional API auth: config flag (e.g. api_auth_token); middleware in api/server.go; exempt /health (and optionally /ready).
- [ ] Optional API TLS: HTTPS for REST API (cert/key paths or ACME); required for internet-exposed API.
- [ ] Optional rate limiting: per-IP (and optionally per-path) for API and/or DNS; configurable.
- [ ] DoT/DoH upstream: support querying upstreams over TLS (DoT port 853) or HTTPS (DoH); config e.g. server type udp|dot|doh, DoH URL.
- [ ] DNS over TCP: add TCP listener on port 53 (RFC 7766); reuse same handler as UDP for large/truncated responses.
- [ ] Amplification mitigation: cap response size vs request and/or require EDNS; avoid replying with much larger payloads to small requests.
- [ ] Configurable bind address: allow binding DNS (and optionally API) to a specific IP (e.g. config dns_bind, api_bind) instead of 0.0.0.0.

---

## 7. DoT/DoH server and DNSSEC

_Inbound_ listeners on dnsplane. For **querying upstreams** over TLS/HTTPS see §6 (DoT/DoH upstream)._

- [ ] DoT server: listen on configurable port (e.g. 853); config dot_enabled, dot_port, dot_cert_file, dot_key_file; new listener in main.go; reuse DNS handler.
- [ ] DoH server: endpoint (e.g. /dns-query); config doh_enabled, doh_path; TLS required; call same resolver as UDP/TCP/DoT.
- [ ] DNSSEC validation: validate upstream responses; set AD bit when valid; config dnssec_validate; optional strict SERVFAIL.
- [ ] DNSSEC signing (optional): sign local zone (dnsrecords.json); key generation, sign on-the-fly or pre-signed.

---

## 8. Package building (RPM, DEB)

- [ ] Add spec file (e.g. packaging/dnsplane.spec): build binary, install to /usr/bin or /usr/local/dnsplane, install systemd unit, optional conffiles and dnsplane user. Version/Release from tag or VERSION file.
- [ ] Add debian/: control, rules, changelog, compat, optional dnsplane.install; rules builds binary, installs binary and systemd unit.
- [ ] Document in README or docs/packaging.md: build and sign with **pbuild** only; no GitHub Actions/cloud CI for building/signing (private keys must not be in GitHub).
- [ ] Optional: APK (Alpine), Homebrew formula (macOS), Windows MSI/zip.

---

## 9. REST API (follow-ups)

- [ ] `GET /dns/records`: optional query params `?name=`, `?type=` for filtering; mirror `dnsrecords.List`.

---

## 10. Later (deferred)

- [ ] Per-server fallback: optional second address when whitelist server fails (resolver + dnsservers).
- [ ] Record PATCH: use identifier (e.g. name+type+value) for update/delete to avoid overwrites.

---

## 11. Resolver performance plan (fast path + indexes)

_Tracked here because the Cursor plan file is not in-repo._

**Phase 1 — host + data path**

- [x] Optional Linux host tuning doc: [`docs/host-tuning.md`](docs/host-tuning.md)
- [x] In-memory indexes for cache + local records (`LookupCacheRR`, `LookupLocalRRs`, rebuild on load/store)
- [x] Short-circuits (empty slices, cache disabled branch)
- [x] Bench / verify latency improvement (measured ~1ms+ gain)

**Phase 2 — unified resolution + observability**

- [x] Single fast path for all QTYPEs: parallel local + cache + upstreams; priority local > cache > first upstream
- [x] PTR: local-first (incl. A→PTR), then fast path on miss
- [x] Resolver perf by query type (`by_query_type` in JSON + perf HTML page)
- [x] README resolution diagram + link to host-tuning

**Optional later**

- [ ] Extra micro-opts if profiling shows a clear target.

---

## 12. Clustered DNS (multi-node sync)

Goal: multiple dnsplane instances that stay in sync for records (and optionally other data) over a custom protocol.

- [ ] Define data to sync: records (dnsrecords) as primary payload; optionally adblock and upstream list; cache per-node, not synced.
- [ ] Custom sync protocol: TCP (optionally TLS), configurable listen port and peer list; message types: full dump, delta (add/update/delete since sequence N), heartbeat.
- [ ] Sequence numbers or version vector per node; authentication: shared secret (cluster_auth_token) or mTLS; wire format: length-prefixed, schema for record + metadata (seq, timestamp).
- [ ] Topology: primary–replica, or multi-primary with conflict resolution, or single writer with leader election (e.g. raft); configurable.
- [ ] Discovery: config list cluster_peers; optional DNS SRV for dynamic discovery.
- [ ] Implementation: sync listener and sync client; apply incoming sync to local store (data.UpdateRecords etc.) and persist; trigger reload. Config: cluster_enabled, cluster_peers, cluster_listen_addr, cluster_auth_token, sync_interval or push-on-change.
- [ ] Docs: deployment (e.g. 2–3 nodes behind load balancer); only one writer in primary–replica to avoid split-brain.

---

## 13. ISPConfig and cPanel compatibility

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
