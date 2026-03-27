# BIND zone files as records source

dnsplane can load DNS records from **BIND-style master zone files** (common with **ISPConfig**, **cPanel**, and stock **BIND**), in addition to JSON `records_source` types `file`, `url`, and `git`.

## Configuration

Set `records_source` in `dnsplane.json`:

```json
"records_source": {
  "type": "bind_dir",
  "location": "/var/lib/dnsplane/zones",
  "include_pattern": "*.db",
  "named_conf": "/etc/bind/named.conf.local",
  "watch": true
}
```

| Field | Meaning |
| --- | --- |
| `type` | Must be `bind_dir`. |
| `location` | Directory to scan, or context for paths from `named_conf`. |
| `include_pattern` | Glob under `location` (default `*.db`). Ignored when `named_conf` is set and yields file paths. |
| `named_conf` | Optional. Subset parser: `zone "name" { ... file "path"; ... }` blocks; loads each `file` path (absolute or relative to the **directory containing** `named_conf`). |
| `watch` | If true, reload when zone files change (`fsnotify`, debounced). |

**Read-only:** Like `url` / `git`, `bind_dir` is **read-only** from dnsplane’s perspective: API/TUI record writes that persist to disk are disabled; use your panel or editor to change zones, then **reload** (watch, `POST /dns/records/reload`, or `record load` in the TUI).

## Supported RR types

Parsed into the internal flat record model:

- **SOA, NS, A, AAAA, CNAME, MX, TXT, PTR**

Other types (e.g. **SRV**, **TLSA**, **DNSSEC** RRs) are **skipped** with a parser warning in logs when loading.

## Limitations

- **`$INCLUDE`** is **disabled** by default in the parser (security / path control). Prefer flattened exports or a single file per zone.
- **Merged zone store:** All zones are merged into one flat list with FQDN-normalized names. Authoritative **delegation** semantics between zones are not modeled separately; answers follow the same name/type matching as JSON records.
- **RFC 2136 dynamic updates** are **not** implemented; UPDATE queries receive **NOTIMP**.

## Reload

- **HTTP:** `POST /dns/records/reload` (requires API auth when `api_auth_token` is set).
- **TUI:** `record load` reloads from the configured source.
- **Filesystem:** optional `watch: true` on `bind_dir`.

## AXFR (optional)

When `axfr_enabled` is true in config, **TCP** (and **DoT** if used) may answer **AXFR** for a zone apex present in the loaded data. Restrict with `axfr_allowed_networks` (CIDR list). See [dnsplane.example.json](dnsplane.example.json) for keys.

## See also

- [ISPConfig notes](ispconfig.md)
- [cPanel notes](cpanel.md)
- [Config files and API](config-files.md)
