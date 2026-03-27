# Roadmap and backlog

Work tracked for future releases and packaging. For **using** dnsplane, see the [README](README.md).

---

## 1. OS packages (RPM, DEB)

- [ ] RPM spec (e.g. `packaging/dnsplane.spec`): install binary, systemd unit, optional dedicated user and config file handling. Version/release from git tag or `VERSION` file.
- [ ] Debian packaging (`debian/`): control, rules, changelog, install paths for binary and unit.
- [ ] Optional: Alpine APK, Homebrew formula, Windows MSI or zip.

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


## 5. Web UI for Configuration
