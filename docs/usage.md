# Usage / examples

dnsplane has two commands: **server** (run the DNS server and TUI/API listeners) and **client** (connect to a running server). The `--config` flag applies only to the **server** command.

**Version:** `/version`, `/ready`, and the TUI banner report the version string baked into the binary you are running.

## start as daemon (server)

```bash
./dnsplane server
```

The daemon keeps the resolver running, exposes a UNIX control socket for the TUI (path depends on OS and user—see **Defaults and non-root** under [config and data file paths](#config-and-data-file-paths)), and listens for remote TUI clients on TCP port `8053` by default.

## start in client mode (connects to the default unix socket unless overridden)

```bash
./dnsplane client
# or specify a custom socket path or address
./dnsplane client /tmp/dnsplane.sock
```

## connect to a remote resolver over TCP (default port 8053)

```bash
./dnsplane client 192.168.178.40
./dnsplane client 192.168.178.40:8053
```

## change the server socket path (server command)

```bash
./dnsplane server --server-socket /tmp/custom.sock
```

## change the TCP TUI listener (server command)

```bash
./dnsplane server --server-tcp 0.0.0.0:9000
```

## config and data file paths

When you do not pass any path flags, dnsplane looks for an existing `dnsplane.json` in the executable directory, then the user config dir (e.g. `~/.config/dnsplane/`), then `/etc/dnsplane.json`. If none is found, it creates the config and data files in the **current directory only** (never in `/etc` or elsewhere). You can override the config file and the JSON data files with server flags:

```bash
./dnsplane server --config ./myconfig.json --dnsrecords ./records.json --cache ./cache.json --dnsservers ./servers.json
```

| Flag | Purpose |
| --- | --- |
| `--config` | Path to config file; if the file does not exist, a default config is created there (server only) |
| `--dnsservers` | Path to dnsservers.json (overrides config) |
| `--dnsrecords` | Path to dnsrecords.json (overrides config) |
| `--cache` | Path to dnscache.json (overrides config) |

If a data file does not exist, dnsplane creates it with default contents at the configured (or overridden) path. When the records source is URL or Git, no local records file is created (records are read-only from the remote source).

**Log directory (defaults):** If your config directory is **not** under `/etc`, the default log folder is `log` next to that directory (e.g. `./log` or `~/.config/dnsplane/log`). If the config directory **is** under `/etc`, the default log directory is `/var/log/dnsplane`.

**UNIX socket (defaults):** The server and client use the same path unless you set `--server-socket`. **Running as root:** the socket is under the OS temp directory (on Linux this is often `/tmp/dnsplane.socket`). **Not root:** `$XDG_RUNTIME_DIR/dnsplane.socket` if set; otherwise `dnsplane/dnsplane.socket` under the user config directory (Linux: typically `~/.config/dnsplane/`; macOS: `~/Library/Application Support/dnsplane/`). That way each unprivileged user gets their own socket by default.

## TUI (interactive client)

When you run `dnsplane client` (or connect over TCP), you get an interactive TUI. Main areas:

- **record** – Add, remove, update, list DNS records (`record add <name> [type] <value> [ttl]`, etc.).
- **dns** – Manage upstream DNS servers: add, update, remove, list, clear, load, save. Use named params: `dns add 1.1.1.1 53`, `dns add 192.168.5.5 53 active:true adblocker:false whitelist:example.com,example.org`.
- **server** – **config** (show all settings), **set** (e.g. `server set apiport 8080`; in-memory until you run **save**), **save** (write config to disk), **load** (reload config from disk), **start** / **stop** (dns, api, or client listeners), **status**, **version**.
- **adblock** – **load** &lt;file or URL&gt; (merge into block list), **list** (loaded sources and counts), **domains** (list blocked domains), **add** / **remove** / **clear**.
- **tools** – **dig** (e.g. `tools dig example.com`, `tools dig example.com @8.8.8.8`).
- **cache** – `cache list`, **`cache clear`** (empty in-memory cache; **`cache save`** to persist `dnscache.json`), `cache load` / `cache save`, `cache remove …`.
- **stats** – Query counts, cache hits, block list size, runtime stats.
- **statistics** – View aggregated data from the full_stats DB: `statistics requesters [full]`, `statistics domains [full]`, **`statistics clear`** (wipe DB + session counters), **`statistics save`** (flush `stats.db` to disk, like `cache save` after `cache clear`). Requires `full_stats: true` in config.

Use `?` or `help` after a command in the TUI for usage.

## Demo (TUI: records)

https://github.com/user-attachments/assets/f5ca52cb-3874-499c-a594-ba3bf64b3ba9
