# Multi-node clustering (DNS records sync)

dnsplane can replicate **local DNS records** (`dnsrecords`) between instances over a **private TCP** connection using a **length-prefixed JSON** protocol. Cache, adblock lists, and upstream server lists are **not** replicated.

## When to use it

- Two or more dnsplane nodes that should serve the **same zone data** (e.g. behind a load balancer).
- Use a **single shared `cluster_auth_token`** on all peers.
- Prefer a **file-backed** `records_source` (not URL/Git-only) when you expect peers to **apply** incoming snapshots; read-only sources reject cluster applies.

## Configuration

Set these keys in `dnsplane.json` (see [README](../README.md) for general config layout):

| Key | Meaning |
|-----|---------|
| `cluster_enabled` | `true` to enable the cluster listener and sync behaviour. |
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

State file **`cluster_state.json`** is created next to `dnsplane.json`. It holds `node_id`, `local_seq`, and per-peer sequence numbers for duplicate detection.

## TUI commands

Run `cluster ?` in the server TUI. Common flows:

| Command | Purpose |
|--------|---------|
| `cluster status` | JSON runtime status (peers, probe RTT, errors). |
| `cluster pull` | Force pull from all configured peers. |
| `cluster join` | Show **node_id**, listen address, **dial address**, and **SHA-256 hex** of `cluster_auth_token` for operator verification. |
| `cluster peer add <host:port> [full\|readonly]` | Add peer to local config; optional **remote** role via `admin_config_apply` (requires `cluster_admin` + `cluster_admin_token`). |
| `cluster peer remove <host:port>` | Remove peer from local `cluster_peers`. |
| `cluster peer set-role <host:port> full|readonly` | Push `cluster_replica_only` to the **remote** node. |
| `cluster push records <host:port>` | Send one full `records_full` snapshot to that peer. |
| `cluster push config <host:port>` | Push `cluster_auth_token` and `cluster_peers` from this node to the peer (for bootstrap). |

`server set` also supports the `cluster_*` keys (then `server save`).

## Web dashboard

The live dashboard (`/stats/dashboard`) includes a **Cluster** panel (read-only) when the process registers a cluster manager. It shows node id, sequence, replica/admin flags, dial address, and a per-peer table (reachability, probe RTT, last error). Management remains in the TUI.

## Remote admin protocol

After `auth_ok`, a client may send `admin_config_apply` with:

- `admin_token` — must equal the target’s `cluster_admin_token`.
- Optional fields: `cluster_auth_token`, `cluster_peers`, `cluster_replica_only`, `cluster_reject_local_writes`, `cluster_admin`, `cluster_sync_interval_seconds`.

The target persists via `data.UpdateSettings`. **REST API does not** expose config mutation; admin is TCP-only.

## Bootstrapping a new node

1. On the **new** server: enable cluster, set `cluster_auth_token` to the same value as the cluster, `cluster_listen_addr`, and optionally `cluster_admin_token` (same as other admins if you want remote role/config pushes).
2. Run **`cluster join`** and copy **dial_address** (and verify **token_sha256_hex** matches the shared secret).
3. On a **full** admin node: **`cluster peer add <dial_address> [readonly]`** to add the peer and optionally set replica mode remotely; then **`cluster push config <dial_address>`** and **`cluster push records <dial_address>`** to align token, peers, and records.
4. On the **new** node: add the full server(s) to `cluster_peers` (or receive them via `push config`) so pulls/pushes are symmetric.

## Protocol (summary)

1. **Frames:** `uint32` big-endian length + UTF-8 JSON body (max 64 MiB per frame).
2. **Auth:** first client message after connect is `{"type":"auth","token":"<cluster_auth_token>"}`; server responds with `auth_ok` or `auth_fail`.
3. **Messages:** `records_full`, `pull`, `ping` / `pong`, `admin_config_apply` / `admin_config_ok` / `admin_config_fail`, `error`.
4. **Ordering:** Each node maintains a monotonic **local sequence**; peers track **last seen** sequence per node id to avoid re-applying the same snapshot.

Traffic is **not TLS** by default. Run the cluster port only on a **trusted network** or tunnel (e.g. VPN, SSH, WireGuard).

## Deployment notes

- **Split-brain:** If two nodes edit records independently, **last writer wins** by sequence; there is no CRDT or Raft election in this release.
- **Load balancer:** Point DNS clients at the VIP; **cluster sync** is a separate TCP port between nodes—do not expose it to the public Internet without protection.
- **Single writer:** For predictable behaviour, prefer **one** admin path (e.g. TUI on one node) or accept occasional overwrites.

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
