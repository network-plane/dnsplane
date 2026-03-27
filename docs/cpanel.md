# cPanel / WHM and dnsplane

cPanel servers often keep BIND zone masters under **`/var/named/`** (file names vary; many use the domain name as the file base). WHM’s DNS zone editor updates those files; BIND is reloaded via cPanel’s tooling (conceptually similar to **`rndc reload`**).

## Pointing dnsplane at cPanel zones

1. Set **`records_source.type`** to **`bind_dir`** and **`location`** to **`/var/named`** (or the path your server uses).
2. Use **`include_pattern`** if only a subset of files should load (default **`*.db`** may or may not match your naming; adjust the glob to match your zone files).
3. When zones change on disk, reload dnsplane with **`watch: true`**, **`POST /dns/records/reload`**, or TUI **`record load`**.

## WHM / nameserver roles

dnsplane can sit in front as a caching or hybrid resolver that also serves **local** answers from merged zones, or act as a read-only mirror of panel-managed data. The exact topology (authoritative vs forwarding) depends on how you point NS glue and which service listens on port 53; this document only covers **loading** cPanel-style zone files.

## Security

- Prefer **`api_auth_token`** when exposing the REST API.
- If **`axfr_enabled`** is true, set **`axfr_allowed_networks`** to the secondary servers’ addresses only.

See **[zone-files.md](zone-files.md)** and **[config-files.md](config-files.md)** for configuration details.
