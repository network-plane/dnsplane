# Roadmap and backlog

Work tracked for future releases and packaging. For **using** dnsplane, see the [README](README.md).

---

## 1. OS packages (RPM, DEB)

- [ ] RPM spec (e.g. `packaging/dnsplane.spec`): install binary, systemd unit, optional dedicated user and config file handling. Version/release from git tag or `VERSION` file.
- [ ] Debian packaging (`debian/`): control, rules, changelog, install paths for binary and unit.
- [ ] Packaging notes: how to build and sign packages locally; keep signing keys off shared CI unless you use project-specific secret storage.
- [ ] Optional: Alpine APK, Homebrew formula, Windows MSI or zip.

---

## 2. API and resolver enhancements

- [ ] `GET /dns/records`: optional `?name=` / `?type=` filters aligned with the TUI list behavior.
- [ ] Per-server fallback: optional second upstream when a whitelist-only server fails.
- [ ] Record updates by stable id (name + type + value) to avoid accidental overwrites.
- [ ] Performance work driven by profiling results if a clear bottleneck appears.

---

## 3. Clustered DNS (multi-node sync)

- [ ] Stronger topology options than last-writer-wins: primary/replica, Raft, or configurable conflict rules.
- [ ] Optional peer discovery (e.g. DNS SRV) instead of only static `cluster_peers`.

---

## 4. ISPConfig and cPanel compatibility

**Goal:** Run dnsplane where panels already manage zones (BIND/PowerDNS), reading zone files the panel maintains.

- [ ] BIND zone parser: `$ORIGIN`, `$TTL`, SOA, NS, A, AAAA, CNAME, MX, TXT, PTR, and common types; relative names and multi-line RRs; load into dnsplane’s record store.
- [ ] Multi-zone storage: merged FQDN store or per-zone store with correct NS/SOA and longest-match lookup.
- [ ] `records_source` variant for zone directories (optional `named.conf` mapping); load on startup and reload.
- [ ] Reload: `inotify`/fsnotify on zone directory, or `POST /reload`, or optional `rndc`-style control if required by hosting workflows.
- [ ] Zone transfer: AXFR (and optionally IXFR) if this server must act as a primary for secondaries.
- [ ] ISPConfig-oriented notes: typical zone paths and reload flow.
- [ ] cPanel-oriented notes: `/var/named/` (or equivalent), reload via `rndc` or scripts, WHM nameserver options.
- [ ] Optional RFC 2136 dynamic updates where panels expect them.
- [ ] README sections: “Using dnsplane with ISPConfig” and “Using dnsplane with cPanel” (examples and reload scripts).
