# dnsplane
![dnsplane](https://github.com/user-attachments/assets/38214dcd-ca33-41ce-a88f-7edad7d85822)

A DNS server with a TUI and REST API. For common record types (**A**, **AAAA**, **MX**, …) answers come from **local records**, then **cache** (if enabled), then **upstreams** queried in parallel; the first successful upstream wins. **PTR** uses local data (including PTR synthesized from **A**) first, then the same path on miss.

## Documentation

| Doc | What it covers |
|-----|----------------|
| **[docs/resolution.md](docs/resolution.md)** | **Resolution behavior**: local/cache/upstream order, adblock and whitelist interaction, cache warm/compaction, mermaid flow diagram. |
| **[docs/usage.md](docs/usage.md)** | **Server and client** commands, flags, config/data paths, **TUI** overview, demo. |
| **[docs/config-files.md](docs/config-files.md)** | **JSON files** (`dnsrecords`, `dnsservers`, cache, `dnsplane.json`), **records source** (file/URL/Git), **domain whitelist**, **adblock**, **main config tables**, **REST API**, **curl** (upstream health). |
| **[docs/logging.md](docs/logging.md)** | Log directory, severity, rotation; client `--log-file`. |
| **[docs/systemd.md](docs/systemd.md)** | **systemd** install using `systemd/dnsplane.service`. |
| **[docs/host-tuning.md](docs/host-tuning.md)** | Optional **Linux OS / host tuning** for DNS latency (buffers, limits, containers). |
| **[docs/upstream-health.md](docs/upstream-health.md)** | **Upstream health checks**: probes, marking servers down, config, logs, **curl**. |
| **[docs/clustering.md](docs/clustering.md)** | **Multi-node record sync**: TCP peers, `cluster_*` keys, auth, deployment notes. |
| **[docs/zone-files.md](docs/zone-files.md)** | **BIND zone files** as `records_source` (`bind_dir`), reload, optional AXFR. |
| **[docs/ispconfig.md](docs/ispconfig.md)** | **ISPConfig**: typical zone paths, reload workflow with dnsplane. |
| **[docs/cpanel.md](docs/cpanel.md)** | **cPanel / WHM**: `/var/named`, reload, high-level deployment notes. |
| **[docs/security-public-dns.md](docs/security-public-dns.md)** | DoT / DoH / DNSSEC when exposing DNS to the internet. |
| **[docs/dnsplane.example.json](docs/dnsplane.example.json)** | Full annotated example `dnsplane.json`. |
| **[examples/dnsplane-example.json](examples/dnsplane-example.json)** | Short starter config (DoT/DoH/DNSSEC present, off by default). |
| **[examples/dnsservers-example.json](examples/dnsservers-example.json)** | Example `dnsservers.json` with **`domain_whitelist`**. |
| **[examples/curl-stats-dashboard.sh](examples/curl-stats-dashboard.sh)** | **curl** examples for dashboard endpoints. |
| **[examples/README.md](examples/README.md)** | Index of example files vs **`docs/dnsplane.example.json`**. |
| **[TODO.md](TODO.md)** | Upcoming work. |

DoT, DoH, and DNSSEC are also summarized in the [main config tables](docs/config-files.md#main-config-options-dnsplanejson) in **config-files.md**.

## Features (summary)

- **Adblock:** Load lists from file or URL; merge multiple sources; optional `adblock_list_files` in config.
- **Full stats:** Optional persistent stats DB and TUI `statistics` commands (`full_stats`, `full_stats_dir`).
- **Split DNS:** Per-upstream **`domain_whitelist`** in `dnsservers.json` / TUI.
- **Records from URL or Git:** Read-only remote sources with a refresh interval.
- **TUI server control:** `server config` / `set` / `save`, and start/stop for DNS, API, and client listeners.

Recent releases add graceful shutdown timeouts for systemd, **statistics** in the TUI when `full_stats` is enabled, and build metadata (version, Go, OS, arch) on stats and health endpoints.

## Dependencies & Documentation
[![Known Vulnerabilities](https://snyk.io/test/github/network-plane/dnsplane/badge.svg)](https://snyk.io/test/github/network-plane/dnsplane)
[![Maintainability](https://qlty.sh/gh/network-plane/projects/dnsplane/maintainability.svg)](https://qlty.sh/gh/network-plane/projects/dnsplane)

[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/8887/badge)](https://www.bestpractices.dev/projects/8887)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/network-plane/dnsplane/badge)](https://securityscorecards.dev/viewer/?uri=github.com/network-plane/dnsplane)
[![OpenSSF Baseline](https://www.bestpractices.dev/projects/8887/baseline)](https://www.bestpractices.dev/projects/8887)

![Go CI](https://github.com/network-plane/dnsplane/actions/workflows/go-ci.yml/badge.svg?branch=master)
![govulncheck](https://github.com/network-plane/dnsplane/actions/workflows/govulncheck.yml/badge.svg?branch=master)
![OSV-Scanner](https://github.com/network-plane/dnsplane/actions/workflows/osv-scanner.yml/badge.svg?branch=master)
![ClusterFuzzLite](https://github.com/network-plane/dnsplane/actions/workflows/clusterfuzzlite.yml/badge.svg?branch=master)


[![Go Mod](https://img.shields.io/github/go-mod/go-version/network-plane/dnsplane?style=for-the-badge)]()
[![Go Reference](https://pkg.go.dev/badge/github.com/network-plane/dnsplane.svg)](https://pkg.go.dev/github.com/network-plane/dnsplane)
[![Dependencies](https://img.shields.io/librariesio/github/network-plane/dnsplane?style=for-the-badge)](https://libraries.io/github/network-plane/dnsplane)

![SBOM](https://github.com/network-plane/dnsplane/actions/workflows/sbom.yml/badge.svg?branch=master)


## Contributing

Contributions are welcome. Please follow the [Google Go Style Guide](https://google.github.io/styleguide/go/). Commits use the [Developer Certificate of Origin (DCO)](https://developercertificate.org/)—see [CONTRIBUTING.md](CONTRIBUTING.md) for sign-off. Roles and decisions are described in [GOVERNANCE.md](GOVERNANCE.md).

## Commit Activity
![GitHub last commit](https://img.shields.io/github/last-commit/network-plane/dnsplane)
![GitHub commits since latest release](https://img.shields.io/github/commits-since/network-plane/dnsplane/latest)
![GitHub Issues or Pull Requests](https://img.shields.io/github/issues/network-plane/dnsplane)



## Authors

- [@earentir](https://www.github.com/earentir)


## License

Licensed under [GPL-2.0-only](https://opensource.org/license/gpl-2-0).

[![License](https://img.shields.io/github/license/network-plane/dnsplane)](https://opensource.org/license/gpl-2-0)
