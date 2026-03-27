# Config files and API reference

JSON data files, `dnsplane.json` keys, REST paths, and curl samples. For resolution order and the diagram see [resolution.md](resolution.md). For CLI paths and the TUI see [usage.md](usage.md). For logging options see [logging.md](logging.md).

## Config Files

| File | Usage |
| --- | --- |
| dnsrecords.json | holds dns records |
| dnsservers.json | holds the dns servers used for queries (see [Upstream servers and domain whitelist](#upstream-servers-dnsserversjson-and-domain-whitelist)) |
| dnscache.json | Cached answers while still valid (TTL not expired), when caching is enabled |
| dnsplane.json | the app config |

Starter copies in the repo: **[examples/dnsplane-example.json](../examples/dnsplane-example.json)** and **[examples/dnsservers-example.json](../examples/dnsservers-example.json)** (point `file_locations.dnsservers` at your real path, e.g. `./dnsservers.json`).

### Records source (file, URL, Git, or BIND zone directory)

DNS records are always configured via `file_locations.records_source` in `dnsplane.json`. One source type applies for both loading and (when writable) saving:

- **file** – Local path; records are read from and written to this path (e.g. `dnsrecords.json`). Default.
- **url** – HTTP(S) URL that returns JSON in the same format as the records file (`{"records": [...]}`). Read-only; a refresh interval controls how often dnsplane re-fetches.
- **git** – Git repository URL (HTTPS or SSH). dnsplane clones/pulls the repo and reads `dnsrecords.json` at the **root**. Read-only; refresh interval controls how often it runs `git pull`.
- **bind_dir** – Directory of BIND-style zone files (see [zone-files.md](zone-files.md)). Read-only for API/TUI mutations that would persist to JSON; use **`POST /dns/records/reload`**, **`record load`**, optional **`watch`**, or optional **`refresh_interval_seconds`** to refresh from disk.

For **url**, **git**, and **bind_dir**, record **add/update/delete** via API and TUI returns **403** (records source is read-only). **`POST /dns/records/reload`** and **`record load`** still reload from the configured source.

Example – local file (default):

```json
"file_locations": {
  "dnsservers": "/etc/dnsplane/dnsservers.json",
  "cache": "/etc/dnsplane/dnscache.json",
  "records_source": {
    "type": "file",
    "location": "/etc/dnsplane/dnsrecords.json"
  }
}
```

Example – URL (read-only):

```json
"records_source": {
  "type": "url",
  "location": "https://example.com/dnsrecords.json",
  "refresh_interval_seconds": 60
}
```

Example – Git (read-only):

```json
"records_source": {
  "type": "git",
  "location": "https://github.com/you/repo.git",
  "refresh_interval_seconds": 120
}
```

For **url** and **git**, `refresh_interval_seconds` defaults to 60 if omitted.

Example – BIND zone directory (read-only):

```json
"records_source": {
  "type": "bind_dir",
  "location": "/var/named",
  "include_pattern": "*.db",
  "named_conf": "/etc/named.conf.local",
  "watch": true,
  "refresh_interval_seconds": 300
}
```

Omit `named_conf` to glob `include_pattern` under `location` (default pattern `*.db`). Set `refresh_interval_seconds` only if you want periodic reloads in addition to (or instead of) `watch`.

### Upstream servers (`dnsservers.json`) and domain whitelist

The file is JSON: **`{ "dnsservers": [ ... ] }`**. Each entry has at least `address`, `port`, `active`, `local_resolver`, `adblocker`. Optional: `transport` (`udp` / `tcp` / `dot` / `doh`), `doh_url` for DoH. Optional **per-row fallback** (queried in parallel with that row): `fallback_address`, `fallback_port` (default `53`), `fallback_transport` (defaults to the row’s `transport`), `fallback_doh_url` (for DoH fallbacks).

**`domain_whitelist`** (optional array of strings): if set, this upstream is used **only** for query names that match an entry (exact name or a subdomain under that suffix). Names that match **any** active whitelisted server are resolved **only** via those servers — **not** via global upstreams (no `domain_whitelist`) and **not** via the configured **`fallback_server_*`** for that query. You can list **multiple** suffixes on one server, or use **multiple** server rows each with its own whitelist and IP.

Example (same structure as **[examples/dnsservers-example.json](../examples/dnsservers-example.json)**):

```json
{
  "dnsservers": [
    {
      "address": "1.1.1.1",
      "port": "53",
      "active": true,
      "local_resolver": true,
      "adblocker": false
    },
    {
      "address": "10.0.0.53",
      "port": "53",
      "active": true,
      "local_resolver": true,
      "adblocker": false,
      "domain_whitelist": ["internal.example.com", "corp.other.net"]
    }
  ]
}
```

TUI equivalent for the internal row:
`dns add 10.0.0.53 53 active:true localresolver:true adblocker:false whitelist:internal.example.com,corp.other.net`

See also [resolution.md](resolution.md) for **Domain whitelist** and **Server selection**.

### Adblock lists

Blocked domains are stored in a single in-memory list. You can:

- **TUI:** `adblock load <filepath|url>` (one file or URL per command; each load is merged), `adblock list` (sources and count per source), `adblock domains` (all blocked domains), `adblock add` / `adblock remove` / `adblock clear`.
- **Config:** In `dnsplane.json`, set `adblock_list_files` to an array of paths. Those files are loaded in order at startup and merged into the block list (same hosts-style format: `0.0.0.0 domain1.com domain2.com`). Stats show "Adblock list: N domains".

### Main config options (`dnsplane.json`)

See **[dnsplane.example.json](dnsplane.example.json)** for every key and defaults. **[examples/dnsplane-example.json](../examples/dnsplane-example.json)** is a shorter starter. Upstreams and **`domain_whitelist`** are covered in **[examples/dnsservers-example.json](../examples/dnsservers-example.json)** and [Upstream servers (`dnsservers.json`)](#upstream-servers-dnsserversjson-and-domain-whitelist). The tables below group the same options for quick lookup.

**DNS and fallback**

| Key | Meaning |
| --- | --- |
| `port` | DNS UDP/TCP listen port (default `53`). |
| `dns_bind` | IP to bind for DNS (empty = all interfaces). |
| `fallback_server_ip`, `fallback_server_port` | Resolver used when no upstream matches or for recursion-style paths. |
| `timeout` | Upstream query timeout (seconds). |
| `fallback_server_transport` | `udp` (default), `tcp`, `dot`, or `doh` for the fallback resolver. |
| `dns_refuse_any` | If true, answer NOTIMP for `ANY` queries. |
| `dns_max_edns_udp_payload` | Cap EDNS UDP payload on responses (`0` = unchanged; e.g. `1232`). |

**REST API and client access**

| Key | Meaning |
| --- | --- |
| `api` | Enable REST API listener. |
| `apiport` | API listen port (e.g. `8080`). |
| `api_bind` | API bind address (empty = all interfaces). |
| `api_auth_token` | If set, requires `Authorization: Bearer` or `X-API-Token` except `GET/HEAD /health` and `/ready`. |
| `api_tls_cert`, `api_tls_key` | PEM paths; both set → HTTPS for the API. |
| `api_rate_limit_rps`, `api_rate_limit_burst` | Per-client HTTP rate limit (`0` RPS = disabled). |
| `server_socket` | UNIX socket for TUI control. |
| `server_tcp` | TCP address for remote TUI clients (default `0.0.0.0:8053`). |

**DoT / DoH (inbound)**

| Key | Meaning |
| --- | --- |
| `dot_enabled`, `dot_bind`, `dot_port`, `dot_cert_file`, `dot_key_file` | DNS over TLS listener. |
| `doh_enabled`, `doh_bind`, `doh_port`, `doh_path`, `doh_cert_file`, `doh_key_file` | DNS over HTTPS. |
| `axfr_enabled` | If true, answer **AXFR** over **TCP** and **DoT** for zones present in local records (single-message transfer). |
| `axfr_allowed_networks` | CIDR list allowed to request AXFR (e.g. `["127.0.0.0/8","10.0.0.0/8"]`). If `axfr_enabled` is true but this list is empty or invalid, AXFR is refused. |

**Response / abuse limits**

| Key | Meaning |
| --- | --- |
| `dns_rate_limit_rps`, `dns_rate_limit_burst` | Per-IP DNS rate limit (`0` = off). |
| `dns_amplification_max_ratio` | Max packed response vs request ratio (`0` = off). |
| `dns_response_limit_mode` | `sliding_window` or `rrl`. |
| `dns_sliding_window_seconds`, `dns_max_responses_per_ip_window` | Sliding-window mode. |
| `dns_rrl_max_per_bucket`, `dns_rrl_window_seconds`, `dns_rrl_slip` | RRL mode. |

**DNSSEC**

| Key | Meaning |
| --- | --- |
| `dnssec_validate`, `dnssec_validate_strict`, `dnssec_trust_anchor_file` | Validation when upstream returns DNSSEC material. |
| `dnssec_sign_enabled`, `dnssec_sign_zone`, `dnssec_sign_key_file`, `dnssec_sign_private_key_file` | Sign answers from local records (see [security-public-dns.md](security-public-dns.md)). |

**Cache and performance**

| Key | Meaning |
| --- | --- |
| `cache_records` | Enable resolver cache (`false` = no `dnscache.json` lookups; upstream only). |
| `local_records_enabled` | **Default `true`.** If `false`, **dnsrecords are not used for DNS answers** (forward-only to upstreams + fallback). Records still load for API/TUI unless you use read-only/cluster modes. **`localhost`** is still answered locally (RFC 6761). |
| `min_cache_ttl_seconds` | Floor for cached TTL (default `600`; `0` = use upstream TTL). |
| `stale_while_revalidate` | Serve stale entries (TTL=1) while refreshing in background. |
| `cache_warm_enabled`, `cache_warm_interval_seconds` | Keep-alive self-query (defaults: on, every 10s). |
| `cache_compact_enabled`, `cache_compact_interval_seconds` | Periodic removal of expired cache rows from memory + persist (defaults: on, every 1800s / 30m; interval minimum 60s). No effect if `cache_records` is false. |
| `pretty_json` | **Default `false`.** If `true`, writes **indented** JSON for `dnsservers.json`, `dnsrecords.json` (file source), and `dnscache.json`. If `false`, writes **compact** JSON (less CPU and I/O on large caches). Does not affect `dnsplane.json` itself (the main config file is always written indented when saved). |

**Stats, dashboard, and profiling**

| Key | Meaning |
| --- | --- |
| `stats_perf_page_enabled` | HTML `/stats/perf/page` (default on). |
| `stats_dashboard_enabled` | `/stats/dashboard`, `/stats/dashboard/data`, and `/stats/dashboard/resolutions` (default on). |
| `dashboard_resolution_log_cap` | How many recent DNS resolutions are kept **in memory** for the dashboard **Log** view (default `1000`; max `1000000`). Not persisted; lost on restart. |
| `full_stats`, `full_stats_dir` | Optional aggregated stats DB + TUI `statistics` commands. |
| `pprof_enabled` | **Default `false`.** If `true`, exposes runtime profiling over HTTP (Go pprof: CPU, heap, etc.). |
| `pprof_listen` | Listen address when `pprof_enabled` is true; if empty, **`127.0.0.1:6060`**. In production, keep profiling on loopback or behind a protected interface. |

**Profiling:** When `pprof_enabled` is true, profiling is available on `pprof_listen` (default `127.0.0.1:6060`). Changing `pprof_enabled` or `pprof_listen` takes effect after a restart. **`pretty_json`** affects the next write of data JSON files and can be changed with `server set` + `server save` without restarting.

**Upstream health** — `upstream_health_check_enabled`, `upstream_health_check_failures`, `upstream_health_check_interval_seconds`, `upstream_health_check_query_name`. See [upstream-health.md](upstream-health.md).

**Clustering** — `cluster_enabled`, `cluster_listen_addr`, `cluster_peers`, `cluster_auth_token`, `cluster_node_id`, `cluster_sync_interval_seconds`, `cluster_advertise_addr`, `cluster_replica_only`, `cluster_reject_local_writes`, `cluster_admin`, `cluster_admin_token`, `cluster_sync_policy`, `cluster_allowed_writer_node_ids`, `cluster_discovery_srv`, `cluster_discovery_interval_seconds`. See [clustering.md](clustering.md).

**Files and records**

- `file_locations` — `dnsservers`, `cache`, `records_source` (`type` + `location` + optional `refresh_interval_seconds` for url/git/bind_dir, plus bind_dir-only `include_pattern`, `named_conf`, `watch`). See [Records source](#records-source-file-url-git-or-bind-zone-directory) above. **`dnsservers.json`** format and per-server **`domain_whitelist`**: [Upstream servers (`dnsservers.json`)](#upstream-servers-dnsserversjson-and-domain-whitelist).
- `adblock_list_files` — list of hosts-style list paths loaded at startup.

**`DNSRecordSettings`** — `auto_build_ptr_from_a`, `forward_ptr_queries`, `add_updates_records`.

**`log`** — `log_dir`, `log_severity`, `log_rotation`, `log_rotation_size_mb`, `log_rotation_time_days`.

In the TUI, **`server config`** shows current settings; **`server set <setting> <value>`** then **`server save`** writes them to `dnsplane.json`. For the exact key names as in JSON, see **[dnsplane.example.json](dnsplane.example.json)** (TUI `server set` lists the same keys in its help).

### REST API

Enable the REST API with `"api": true` in `dnsplane.json` and set `apiport` (e.g. `8080`). You can start or stop the API listener from the TUI with `server start api` / `server stop api`.

**Optional API authentication:** set `"api_auth_token": "<secret>"` in `dnsplane.json` (or `server set api_auth_token '<secret>'` then `server save`). When set, clients must send either `Authorization: Bearer <secret>` or `X-API-Token: <secret>`, except **GET/HEAD `/health`** and **GET/HEAD `/ready`** (so automated health checks work without the token). All other paths—including `/version`, `/metrics`, and HTML stats pages—require the token when configured.

**TLS, bind, and rate limits:** set `api_tls_cert` and `api_tls_key` to PEM file paths to serve the REST API over HTTPS. Use `dns_bind` and `api_bind` (e.g. `"127.0.0.1"`) to listen on a specific address instead of all interfaces. Per-IP limits: `api_rate_limit_rps` / `api_rate_limit_burst` (HTTP 429 when exceeded), `dns_rate_limit_rps` / `dns_rate_limit_burst` (DNS `REFUSED` when exceeded). Amplification hardening: `dns_amplification_max_ratio` caps packed response size vs packed request (0 disables).

**Upstream transport:** in `dnsservers.json`, each server may set `transport` to `udp` (default), `tcp`, `dot` (TLS to port 853 by default), or `doh`. For DoH set `doh_url` to the full `https://…/dns-query` URL (or put a URL in `address` when using `doh`). Global config **`fallback_server_*`** applies when the query is **not** using whitelist-only upstreams. Per-row **`fallback_*`** fields add a second resolver in the same parallel race (see [resolution.md](resolution.md)). **`domain_whitelist`** (split DNS) is documented in [Upstream servers (`dnsservers.json`)](#upstream-servers-dnsserversjson-and-domain-whitelist). **`POST` / `PUT` `/dns/servers`** accept the same `fallback_*` JSON keys as `dnsservers.json`.

**Public / internet-facing DNS:** see [security-public-dns.md](security-public-dns.md) for DoT/DoH listeners (`dot_*`, `doh_*`), response limit modes (`dns_response_limit_mode` `sliding_window` vs `rrl`), DNSSEC validation flags, and metrics.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness: returns 200 when the API process is up. No dependency on DNS or other listeners. |
| GET | `/ready` | Readiness: returns 200 when the API and DNS listener are both up, 503 otherwise. Response is JSON with `ready`, `api`, `dns`, `tui_client` (connected, addr, since), `listeners` (dns_port, api_port, api_enabled, client_socket_path, client_tcp_address), and **`build`** (`version`, `go_version`, `os`, `arch`). Use this for load balancers and orchestrator readiness checks (e.g. Kubernetes). |
| GET | `/version` | Build metadata as JSON: `version`, `go_version`, `os`, `arch` (same as `build` in `/stats` and `/ready`). |
| GET | `/version/page` | HTML view of the same build fields (for embedding in the dashboard). **404** if `stats_dashboard_enabled` is false. |
| GET | `/dns/records` | List DNS records (same data as the TUI). Returns JSON with `records`, optional `detailed` (boolean), `filter`, `messages`. **Query:** `name` (substring on normalized owner name), `type` (DNS type, AND with `name` when both set; invalid `type` → **400**), `details` or `d` (`1` / `true` for verbose fields in each record, like `record list details`). |
| POST | `/dns/records` | Add a DNS record. Body: `{"name":"...","type":"A","value":"...","ttl":3600}`. Optional **`id`**: if set, must be unique (for import/sync); if omitted, the server assigns a UUID. Response lists include **`id`** on each record. |
| POST | `/dns/records/reload` | Reload records from the configured `records_source` (no body). Returns `{"status":"reloaded","records":N}`. Subject to **`api_auth_token`** when set. |
| PUT | `/dns/records` | Update a record. With **`id`** in the body, replace that row’s name/type/value/TTL (stable update). Without **`id`**, same as today: match **name + type + value** and update in place (legacy). |
| DELETE | `/dns/records` | Delete by query **`?id=`**… or JSON body `{"id":"..."}`. Otherwise **`name`** (required) plus optional **`type`** / **`value`** (same as legacy). |
| GET | `/dns/servers` | List upstreams plus health: `servers`, `upstream_health_check_enabled`, interval/failures hints, and `upstream_health` per `address_port` (unhealthy, consecutive_failures, last_probe_*, last_success_at). |
| GET | `/dns/upstreams/health` | Same health slice and check settings without full server config. See [upstream-health.md](upstream-health.md). |
| GET | `/stats` | Resolver stats as JSON: `session` / `total` scopes with resolver counters; top-level **`build`** (`version`, `go_version`, `os`, `arch`). When `full_stats` is enabled in config, includes `full_stats.enabled`, `full_stats.requesters_count`, `full_stats.domains_count`. |
| GET | `/metrics` | Prometheus text format: counters and gauges (queries, cache hits, blocks, process uptime, etc.). With `full_stats` enabled, adds full-stats gauges. Histogram **`dnsplane_dns_resolve_duration_seconds`** reports resolve latency by QTYPE (same breakdown as `/stats/perf`). |
| GET | `/stats/dashboard` | Live HTML UI: **Status** (listeners + feature flags), **Statistics** (rates, charts, full_stats top 10, activity log), **Log** (recent resolutions), **Historical** (full_stats), plus nav links for embedded **Tuning** (perf), **Metrics**, **Version**. **404** if `stats_dashboard_enabled` is false (default is on). |
| GET | `/stats/dashboard/data` | JSON backing the dashboard (`counters`, `perf`, `summary`, **`status`**, `series`, `log`, **`per_sec_rates`**, **`fullstats`** / **`fullstats_top`** when `full_stats` is on). **404** if `stats_dashboard_enabled` is false. |
| GET | `/stats/dashboard/resolutions` | JSON for the dashboard **Log** (resolutions): `cap` (matches `dashboard_resolution_log_cap`), `count`, `resolutions` (newest first; client IP, query, type, outcome, upstream, reply, `duration_ms`, time). **404** if `stats_dashboard_enabled` is false. |
| POST | `/stats/dashboard/resolutions/purge` | Clears the in-memory resolution log (same data as the dashboard **Log** and main dashboard activity list). **404** if `stats_dashboard_enabled` is false. |
| GET | `/stats/perf` | JSON performance breakdown: outcomes (local/cache/upstream/none) and histograms for cache-only vs upstream paths. Prefer cache-only vs upstream histograms for tuning; the combined total histogram mixes both. Reset with `POST /stats/perf/reset`. |
| GET | `/stats/perf/page` | HTML view of `/stats/perf` (auto-refresh, reset button). **404** if `stats_perf_page_enabled` is false (default is on). |
| POST | `/stats/perf/reset` | Clears A-record performance counters (use before measuring latency). |

### curl examples (upstream health)

With API on port **8080** (set `apiport` in config):

```bash
# Full upstream health JSON
curl -sS http://127.0.0.1:8080/dns/upstreams/health | jq .

# Servers plus embedded health
curl -sS http://127.0.0.1:8080/dns/servers | jq '{servers: .servers, checks_enabled: .upstream_health_check_enabled, upstream_health: .upstream_health}'
```

Enable checks in `dnsplane.json`, then restart server:

```json
"upstream_health_check_enabled": true,
"upstream_health_check_failures": 3,
"upstream_health_check_interval_seconds": 30,
"upstream_health_check_query_name": "google.com."
```
