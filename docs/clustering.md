# Multi-node clustering (DNS records sync)

dnsplane can replicate **local DNS records** (`dnsrecords`) between instances over a **private TCP** connection using a **length-prefixed JSON** protocol. Cache, adblock lists, and upstream server lists are **not** replicated.

## When to use it

- Two or more dnsplane nodes that should serve the **same zone data** (e.g. behind a load balancer).
- Use a **single shared `cluster_auth_token`** on all peers.
- Prefer a **file-backed** `records_source` (not URL/Git-only) when you expect peers to **apply** incoming snapshots; read-only sources reject cluster applies.

## Configuration

Set these keys in `dnsplane.json` (general layout: [README](../README.md#config-files)):

| Key | Meaning |
|-----|---------|
| `cluster_enabled` | `true` to enable the cluster listener and sync behavior. |
| `cluster_listen_addr` | TCP listen address (e.g. `:7946`). If empty, defaults to `:7946`. |
| `cluster_advertise_addr` | Optional `host:port` shown in `cluster join` when listen is `:7946` (otherwise guessed IPv4 + port). |
| `cluster_peers` | JSON array of `host:port` strings to **push** to after local record changes, and to **pull** from when periodic sync is enabled. |
| `cluster_auth_token` | Shared secret; must match on every peer. |
| `cluster_node_id` | Optional stable node id (string). If empty, a value is generated and stored in `cluster_state.json`. |
| `cluster_sync_interval_seconds` | If set &gt; `0`, periodically **pull** full snapshots from peers. `0` disables periodic pull (push-on-change still runs when peers are listed). |
| `cluster_replica_only` | `true`: **read replica** — applies incoming snapshots but **does not push** to peers. |
| `cluster_reject_local_writes` | `true`: reject **local** record edits (TUI/API); cluster-applied records still work. |
| `cluster_admin` | `true`: this node may send **admin** messages (`admin_config_apply`) to peers. |
| `cluster_admin_token` | Shared secret for **remote admin**; must be **non-empty** on a peer to accept `admin_config_apply`. Same value should be configured on admin nodes that push config/roles. |
| `cluster_sync_policy` | How incoming `records_full` snapshots are accepted: `lww_per_node` (default), `primary_writer`, `global_lww`. See **Sync policies** below. |
| `cluster_allowed_writer_node_ids` | JSON array of `node_id` strings. Required for `primary_writer`: only snapshots whose sender `node_id` is listed are applied. If the policy is `primary_writer` and this list is empty, **all** incoming snapshots are rejected (with a startup warning). |
| `cluster_discovery_srv` | Optional DNS SRV query name, RFC 2782 form, e.g. `_dnsplane._tcp.example.com.` Resolved targets are **merged** with `cluster_peers` for push, pull, and probes; SRV results are not written to `dnsplane.json`. |
| `cluster_discovery_interval_seconds` | Seconds between SRV refreshes when `cluster_discovery_srv` is set. If the key is **omitted** but SRV is set, defaults to **60**. Use **`0`** to resolve SRV **only at startup** (no periodic refresh). |

State file **`cluster_state.json`** is created next to `dnsplane.json`. It holds `node_id`, `local_seq`, per-peer sequence numbers for duplicate detection, and (for `global_lww`) `last_global_lww_unix` / `last_global_lww_node_id`.

## Sync policies

| Policy | Behavior |
|--------|----------|
| `lww_per_node` (default) | Same as historical behavior: each sender has a monotonic sequence; duplicates are skipped. **Different nodes** can still overwrite each other—whoever applies last replaces the full local record store. Safe when only **one** node performs writes. |
| `primary_writer` | Only snapshots from a `node_id` in `cluster_allowed_writer_node_ids` are applied; others are ignored (peer sequence is still advanced so the same frame is not retried forever). |
| `global_lww` | One logical “winner” per snapshot: compares `timestamp_unix` on the message (pushes set this to send time). Newer timestamp wins; if equal, **lexicographic** `node_id` breaks ties. Persisted across restarts. **Clock skew** between nodes can cause the wrong snapshot to win—use reliable time sync (NTP). |

**Interaction with roles:** `cluster_replica_only` and `cluster_reject_local_writes` still apply as before. Prefer **single writer** + `primary_writer` or `global_lww` when multiple nodes might otherwise edit records.

## SRV discovery

- Example record: `_dnsplane._tcp.example.com. IN SRV 0 5 7946 node1.internal.`
- If lookup fails, the node keeps the **last successful** merge for static peers only on first failure after startup; after a failed refresh, `discovery_last_error` is set on dashboard/TUI status until the next successful lookup.
- **Static** `cluster_peers` and **SRV** targets are **deduplicated** (`host:port`); static entries appear first in the merged order.

## TUI commands

**Cluster** is a TUI **context**: type `cluster` to enter it, then run subcommands without repeating `cluster` (same pattern as `dns`, `server`, etc.). You can also run a one-liner from the root prompt, e.g. `cluster status`.

Inside the **cluster** context, `?` / `help` on a command shows usage. Typical commands:

| In context | One-liner from root | Purpose |
|------------|---------------------|---------|
| `status` | `cluster status` | JSON runtime status (peers, probe RTT, errors). |
| `pull` or `sync` | `cluster pull` | Force pull from all configured peers. |
| `join` or `info` | `cluster join` | **node_id**, listen address, **dial address**, **SHA-256 hex** of `cluster_auth_token`. |
| `peer add …` | `cluster peer add …` | Add peer locally; optional **remote** role via `admin_config_apply` (requires `cluster_admin` + `cluster_admin_token`). |
| `peer remove …` | `cluster peer remove …` | Remove peer from local `cluster_peers`. |
| `peer set-role …` | `cluster peer set-role …` | Push `cluster_replica_only` to the **remote** node. |
| `push records …` | `cluster push records …` | Send one full `records_full` snapshot to that peer. |
| `push config …` | `cluster push config …` | Push `cluster_auth_token` and `cluster_peers` (bootstrap). |

`server set` also supports the `cluster_*` keys (then `server save`).

## Web dashboard

The live dashboard (`/stats/dashboard`) includes a **Cluster** panel (read-only) when the process registers a cluster manager. It shows node id, sequence, sync policy, optional global LWW winner, discovery SRV / last refresh / error, static vs SRV peer counts, replica/admin flags, dial address, and a per-peer table (reachability, probe RTT, last error). Management remains in the TUI.

## Remote admin protocol

After `auth_ok`, a client may send `admin_config_apply` with:

- `admin_token` — must equal the target’s `cluster_admin_token`.
- Optional fields: `cluster_auth_token`, `cluster_peers`, `cluster_replica_only`, `cluster_reject_local_writes`, `cluster_admin`, `cluster_sync_interval_seconds`.

The target persists via `data.UpdateSettings`. **REST API does not** expose config mutation; admin is TCP-only.

## Bootstrapping a new node

1. On the **new** server: enable cluster, set `cluster_auth_token` to the same value as the cluster, `cluster_listen_addr`, and optionally `cluster_admin_token` (same as other admins if you want remote role/config pushes).
2. Run **`join`** (in cluster context) or **`cluster join`** from root; copy **dial_address** (and verify **token_sha256_hex** matches the shared secret).
3. On a **full** admin node: **`peer add <dial_address> [readonly]`** (or **`cluster peer add …`** from root) to add the peer and optionally set replica mode remotely; then **`push config <dial_address>`** and **`push records <dial_address>`** to align token, peers, and records.
4. On the **new** node: add the full server(s) to `cluster_peers` (or receive them via `push config`) so pulls/pushes are symmetric.

## Protocol (summary)

1. **Frames:** `uint32` big-endian length + UTF-8 JSON body (max 64 MiB per frame).
2. **Auth:** first client message after connect is `{"type":"auth","token":"<cluster_auth_token>"}`; server responds with `auth_ok` or `auth_fail`.
3. **Messages:** `records_full`, `pull`, `ping` / `pong`, `admin_config_apply` / `admin_config_ok` / `admin_config_fail`, `error`.
4. **Ordering:** Each node maintains a monotonic **local sequence**; peers track **last seen** sequence per sender `node_id` to avoid re-applying the same snapshot. **`cluster_sync_policy`** may further restrict or order applies (see **Sync policies**).

Traffic is **not TLS** by default. Run the cluster port only on a **trusted network** or tunnel (e.g. VPN, SSH, WireGuard).

## Deployment notes

- **Split-brain:** With default `lww_per_node`, two writers can still overwrite each other. Use **`primary_writer`** or **`global_lww`** to tighten behavior. **Raft** or similar consensus is not implemented in this release.
- **Load balancer:** Point DNS clients at the VIP; **cluster sync** is a separate TCP port between nodes—do not expose it to the public Internet without protection.
- **Single writer:** For predictable behavior, prefer **one** admin path (e.g. TUI on one node) or accept occasional overwrites.

## Example (two nodes)

**Node A** (`192.168.1.10`):

```json
"cluster_enabled": true,
"cluster_listen_addr": ":7946",
"cluster_peers": ["192.168.1.11:7946"],
"cluster_auth_token": "change-me-shared-secret",
"cluster_sync_interval_seconds": 60
```

**Node B** (`192.168.1.11`):

```json
"cluster_enabled": true,
"cluster_listen_addr": ":7946",
"cluster_peers": ["192.168.1.10:7946"],
"cluster_auth_token": "change-me-shared-secret",
"cluster_sync_interval_seconds": 60
```

After editing records on a node, peers are notified via **push**; periodic **pull** helps recover if a push was missed.
