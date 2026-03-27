# ISPConfig and dnsplane

ISPConfig typically stores BIND zones under paths such as **`/etc/bind/`** or distribution-specific trees, with zone data in **`*.db`** (or similar) files that BIND reloads after changes.

## Pointing dnsplane at panel zones

1. Set **`records_source.type`** to **`bind_dir`** and **`location`** to the directory that contains your zone master files (or a dedicated copy if you mirror files).
2. If file names are not a simple glob, set **`named_conf`** to a fragment that contains `zone "..." { ... file "..."; }` blocks; dnsplane resolves relative `file` paths against the directory containing that config file.
3. After ISPConfig or BIND reloads zones on disk, trigger a dnsplane reload using one of:
   - **`watch: true`** on `bind_dir` (debounced directory notifications), or
   - **`POST /dns/records/reload`** (with API auth if configured), or
   - TUI **`record load`**.

## Operational notes

- dnsplane merges all zones into one flat record list; use **[zone-files.md](zone-files.md)** for supported RR types and limits.
- Treat **`bind_dir`** as **read-only** from dnsplane: edit zones with ISPConfig/BIND, then reload dnsplane as above.
- **`axfr_enabled`** is optional; if you enable AXFR for secondaries, restrict **`axfr_allowed_networks`** to trusted CIDRs.

See also **[cpanel.md](cpanel.md)** for cPanel/WHM layout.
