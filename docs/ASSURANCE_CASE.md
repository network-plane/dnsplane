# Assurance case (dnsplane)

This document describes the threat model, trust boundaries, and how common risks are mitigated. It supports the [OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/) assurance-case requirement.

## Who should read this

Operators and auditors reviewing design and risk posture. To report vulnerabilities, use [SECURITY.md](../SECURITY.md).

---

## 1. Threat model

### 1.1 Product scope

dnsplane is a DNS server and management stack that:

- Accepts DNS queries (UDP/TCP) from clients and resolves them using local records, cache, optional adblock, and configured upstream DNS servers.
- Exposes a REST API (optional) for reading/updating records and servers and for health, stats, and metrics.
- Exposes an interactive TUI over a UNIX socket or TCP for configuration and operations.
- Reads configuration and data from local JSON files or, optionally, from HTTP(S) URLs or Git repositories (records and adblock lists).

### 1.2 Assets

- **Confidentiality:** Local DNS records and query patterns; configuration (paths, ports, optional API); operational data (stats, full_stats if enabled).
- **Integrity:** DNS responses served to clients; stored records, cache, and upstream server list; configuration and log files.
- **Availability:** DNS resolution service; API and TUI management interfaces.

### 1.3 Threat actors and threats

| Actor / source        | Threat |
|------------------------|--------|
| **Untrusted DNS clients** | Malicious or malformed query names/types; query flooding (DoS); attempts to influence resolution or exhaust resources. |
| **Untrusted API clients** | Malicious or malformed JSON and query parameters; injection or oversized payloads; enumeration or DoS. |
| **Untrusted TUI clients** | Malicious command input over socket/TCP; session takeover or lock contention. |
| **Upstream DNS servers** | Spoofed or malicious responses; cache poisoning if responses are cached without validation. |
| **Remote data sources** | Compromised or malicious content from records URL or Git, or adblock file/URL; malformed JSON or list format. |
| **Local filesystem** | Tampered or poorly permissioned config/data files; symlink or path traversal if paths are not constrained. |
| **Dependencies** | Known vulnerabilities in Go modules or runtime. |

### 1.4 Assumptions

- The process runs with filesystem and network access consistent with deployment (e.g. config and data paths are under operator control).
- Upstream DNS servers and optional remote URLs are chosen by the deployer; dnsplane treats their responses and content as untrusted.
- When the API or TUI is exposed to a network, the network is assumed hostile unless otherwise secured (e.g. network segmentation or auth).

---

## 2. Trust boundaries

Trust boundaries are listed so cross-boundary data is treated as untrusted and validated.

| Boundary | Inside (trusted) | Outside (untrusted) | Data crossing boundary |
|----------|------------------|----------------------|--------------------------|
| **DNS network** | Resolver logic, local records, cache, adblock state | DNS clients, upstream DNS servers | Query packets (names, types, flags); upstream response packets. |
| **REST API** | Daemon state, data layer, business logic | HTTP clients | Request method, path, query params, headers, body (JSON). |
| **TUI (UNIX socket / TCP)** | Daemon state, command handler | TUI clients | Raw input lines (commands and arguments). |
| **Config and data files** | Paths and format expected by the app | File contents on disk | JSON and text read/written at configured paths. |
| **Remote records/adblock** | Parsers and in-memory structures | URL/Git content | HTTP response bodies; Git repo files (e.g. `dnsrecords.json`, adblock lists). |
| **Process boundary** | Single process, goroutines | Other processes, OS | Only via defined interfaces (sockets, files); no execution of external commands from user input. |

Important points:

- **No shell or external execution:** User or API input is never passed to shell or `exec`; there is no command injection boundary.
- **Single process:** No inter-process trust boundary inside the application; concurrency is handled with standard Go synchronization.
- **Explicit interfaces:** Only the DNS listener, HTTP server, TUI socket/TCP, file I/O, and HTTP/Git fetchers cross trust boundaries; all are identified and subject to validation and error handling.

---

## 3. Secure design principles applied

The following principles are applied to keep the design aligned with the threat model and trust boundaries.

- **Least privilege**
  - Config and data file paths are explicit (config file and flags); no arbitrary path injection from network or API.
  - API can be disabled; TUI can be bound to a UNIX socket only (no TCP). **`api_auth_token`** limits who can use the REST API when set (see the main README for which routes stay unauthenticated for health checks).
  - File creation (e.g. socket dir, log dir) uses restricted permissions (e.g. `0o700` for socket directory) where applicable.

- **Defense in depth**
  - Input is validated at the boundary (DNS names, record types/values, IPs, JSON schema, command parsing) before use in business logic or storage.
  - Outputs (e.g. DNS responses, API JSON) are produced from validated internal state; no direct concatenation of untrusted input into protocol or file output.

- **Fail secure**
  - Parse or validation failures (DNS, JSON, record value, adblock format) result in rejecting the operation or the data load, not in applying bad data.
  - Missing or invalid config leaves the process in a predictable state (e.g. defaults or no start) rather than proceeding with unsafe assumptions.

- **Minimal attack surface**
  - No optional scripting or plugins; no dynamic code loading.
  - Capabilities such as the REST API, full_stats, and remote record sources are used only when you enable them in configuration, so unused features stay disabled.

- **Secure defaults**
  - API is off by default; TUI over TCP is opt-in; logging can be set to `none`; default socket and log paths are user- or config-specific to avoid sharing between unrelated installs.

These choices show up as validation on DNS names and record data, structured API request handling, typed config loading, and no shell/`exec` paths driven by network or TUI input.

---

## 4. Common implementation security weaknesses countered

Typical classes of weaknesses are addressed as follows.

- **Injection (command, code, path)**
  - **Mitigation:** User and API input is never passed to shell or `exec`. Config and data paths come from the config file and CLI flags, not from request bodies. Domain names and record values are validated before use in resolution or storage.
  - **Outcome:** No shell or `exec` driven by DNS, API, or TUI input; paths stay within configured locations, with sanitization for remote Git cache directories.

- **Insecure deserialization**
  - **Mitigation:** All JSON uses the standard `encoding/json` decoder. API request bodies are bound to defined structs and validated; config and data files use typed structs. Untrusted data is not unmarshaled into open-ended types. Record and adblock list formats are constrained (e.g. hosts-style lines for adblock).
  - **Outcome:** Deserialization stays type-safe and validated; deserialization-based abuse is unlikely.

- **Sensitive data exposure**
  - **Mitigation:** Log level and paths are configurable; secrets are not logged by default. The API does not return raw config files; **`api_auth_token`** must be supplied by clients and should not appear in logs when used as intended.
  - **Outcome:** Accidental exposure via logs or API responses is limited by design and configuration.

- **Dependency and supply-chain risks**
  - **Mitigation:** Dependencies are managed with Go modules (`go.mod`/`go.sum`). CI runs dependency and vulnerability checks (e.g. OSV-Scanner, govulncheck). Fuzz tests (ClusterFuzz Lite) target parsing and validation code paths.
  - **Outcome:** Known vulnerabilities and regressions are caught in CI; fuzzing improves robustness of parsing and validation.

- **Denial of service (resource exhaustion)**
  - **Mitigation:** DNS and API handlers do not perform unbounded work per request; upstream DNS uses timeouts and parallel query limits. TUI session is single-active with lock; API does not expose expensive operations without limit. Log rotation and configurable timeouts bound long-running work.
  - **Outcome:** DoS from a single client or request is mitigated by timeouts, concurrency limits, and bounded work per request.

- **Insufficient validation**
  - **Mitigation:** DNS names and record types/values are validated before storage or use in resolution. API inputs (e.g. record name/type/value, server address) are validated and rejected when invalid. TUI commands are parsed and checked before execution. URL/Git content is parsed and merged into in-memory structures with validation; invalid entries are rejected.
  - **Outcome:** Invalid or malicious input is rejected at the boundary, reducing impact on resolution and stored data.

This assurance case is maintained with the project and updated when significant new features or trust boundaries are introduced. Security issues can be reported as described in [SECURITY.md](../SECURITY.md).
