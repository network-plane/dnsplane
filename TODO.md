# dnsplane TODO

---

## 1. Package building (RPM, DEB)

- [ ] Add spec file (e.g. packaging/dnsplane.spec): build binary, install to /usr/bin or /usr/local/dnsplane, install systemd unit, optional conffiles and dnsplane user. Version/Release from tag or VERSION file.
- [ ] Add debian/: control, rules, changelog, compat, optional dnsplane.install; rules builds binary, installs binary and systemd unit.
- [ ] Document in README or docs/packaging.md: build and sign with **pbuild** only; no GitHub Actions/cloud CI for building/signing (private keys must not be in GitHub).
- [ ] Optional: APK (Alpine), Homebrew formula (macOS), Windows MSI/zip.

---

## 2. Various

- [ ] `GET /dns/records`: optional query params `?name=`, `?type=` for filtering; mirror `dnsrecords.List`.
- [ ] Per-server fallback: optional second address when whitelist server fails (resolver + dnsservers).
- [ ] Record PATCH: use identifier (e.g. name+type+value) for update/delete to avoid overwrites.
- [ ] Extra micro-opts if profiling shows a clear target.

---

## 3. Clustered DNS (multi-node sync)

- [ ] Topology beyond current LWW: explicit primary–replica mode, Raft/leader election, or stronger conflict policies (configurable).
- [ ] Discovery: optional **DNS SRV** (or similar) for dynamic peer discovery (today: static `cluster_peers` only).

---

## 4. ISPConfig and cPanel compatibility

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
