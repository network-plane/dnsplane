# dnsplane TODO

---

## 1. REST API (data only — no config endpoints)

- **Records**
  - Add `PUT /dns/records` (or PATCH): update by name+type+value. Wire to dnsrecords + data.UpdateRecords.
  - Add `DELETE /dns/records`: identify by name, type, value (query or body). Wire to dnsrecords remove + data.UpdateRecords.
- **DNS servers**
  - Add `POST /dns/servers`: body address, port, active, local_resolver, adblocker, domain_whitelist. Validate via dnsservers; persist via data.SaveDNSServers.
  - Add `PUT /dns/servers/{address}`: update existing server.
  - Add `DELETE /dns/servers/{address}`.
- **Adblock**
  - Add `GET /adblock/domains` (optional: `GET /adblock/sources`).
  - Add `POST /adblock/domains`: body single domain or list.
  - Add `DELETE /adblock/domains`: body or query domain/list.
  - Add `POST /adblock/clear` or `DELETE /adblock/domains` for all.
- **Cache**
  - Add `GET /cache` (read-only).
  - Add `POST /cache/clear` or `DELETE /cache`.

No endpoints for dnsplane.json or any config.

---

## 2. Security

- Optional API auth: config flag (e.g. api_auth_token); middleware in api/server.go; exempt /health (and optionally /ready).
- Optional rate limiting: per-IP (and optionally per-path) for API and/or DNS; configurable.
- DoT/DoH upstream: support querying upstreams over TLS (DoT port 853) or HTTPS (DoH); config e.g. server type udp|dot|doh, DoH URL.

---

## 3. DoT/DoH server and DNSSEC

- DoT server: listen on configurable port (e.g. 853); config dot_enabled, dot_port, dot_cert_file, dot_key_file; new listener in main.go; reuse DNS handler.
- DoH server: endpoint (e.g. /dns-query); config doh_enabled, doh_path; TLS required; call same resolver as UDP/TCP/DoT.
- DNSSEC validation: validate upstream responses; set AD bit when valid; config dnssec_validate; optional strict SERVFAIL.
- DNSSEC signing (optional): sign local zone (dnsrecords.json); key generation, sign on-the-fly or pre-signed.

---

## 4. Testing and CI

- Logger tests: unit tests for pure helpers in logger/logger.go (severity, path); mock file I/O.
- Commandhandler tests: parsing/validation for TUI commands (server add/update, record add/remove); mirror dnsservers apply_test style.
- Integration test: add one more resolver test (e.g. no local/cache/upstream → NXDOMAIN or empty).
- CI: workflow .github/workflows/test.yml — `make test`, `make vet` on push/PR. Tests/vet only; no package build/sign in GitHub (use pbuild).

---

## 5. Observability

- /metrics: add histograms/summaries for DNS latency; optional label by query type.
- Structured logging: add fields (query name, type, upstream, duration) in DNS layer.
- Expose build/version in /stats and optionally /ready or /version (Go version, OS, arch).

---

## 6. Documentation and dev

- .gitignore: narrow `*.json` to specific names or use examples/ so example configs can be committed.
- Add example config (e.g. examples/dnsplane.json or dnsplane-example.json); ensure not ignored.
- CONTRIBUTING: add line that new code should include tests; `make test` and `make fuzz` expected.

---

## 7. Package building (RPM, DEB)

- Add spec file (e.g. packaging/dnsplane.spec): build binary, install to /usr/bin or /usr/local/dnsplane, install systemd unit, optional conffiles and dnsplane user. Version/Release from tag or VERSION file.
- Add debian/: control, rules, changelog, compat, optional dnsplane.install; rules builds binary, installs binary and systemd unit.
- Build and sign with **pbuild** only; do not use GitHub Actions or cloud CI for building/signing (private keys must not be in GitHub). Document in README or docs/packaging.md.
- Optional: APK (Alpine), Homebrew formula (macOS), Windows MSI/zip.

---

## 8. Later

- Per-server fallback: optional second address when whitelist server fails (resolver + dnsservers).
- Record PATCH: use identifier (e.g. name+type+value) for update/delete to avoid overwrites.

---

## 9. Other

- Upstream health: expose last success/failure per server in API (e.g. /dns/servers or /dns/upstreams); use LastUsed/LastSuccess from DNSServer; optional in stats dashboard.
- GET /dns/records: optional query params ?name=, ?type= for filtering; mirror dnsrecords.List.
- README: mention AAAA and PTR in resolution; document DoT/DoH/DNSSEC when implemented.

---

## 10. DNS server health checks

- Optional proactive health checks: periodically test each DNS server (e.g. send a test query and check reply).
- Config: enable/disable (e.g. `upstream_health_check_enabled`), number of consecutive failures before marking server down (e.g. `upstream_health_check_failures`), check interval (e.g. `upstream_health_check_interval`).
- When a server is unreachable: put it on notice, notify the user (log and/or TUI/API), and disable it for forwarding until it is available again; re-check on the same interval and re-enable when it responds.
