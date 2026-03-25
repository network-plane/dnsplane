# Upstream DNS health checks

When **`upstream_health_check_enabled`** is `true` in `dnsplane.json`, dnsplane periodically sends an **A** query (default QNAME `google.com.`) to each **active** upstream. After **N** consecutive probe failures (default **3**), that upstream is **excluded** from forwarding until a probe succeeds again or a **real client query** succeeds via that upstream (which clears the unhealthy state).

If **every** upstream would be excluded, dnsplane **falls back** to using the full server list so resolution does not go completely dark.

## Config (`dnsplane.json`)

| Field | Meaning |
|-------|---------|
| `upstream_health_check_enabled` | `true` to enable probes and filtering. |
| `upstream_health_check_failures` | Consecutive failures before marking down (default **3** if unset or `0`). |
| `upstream_health_check_interval_seconds` | Seconds between probe **rounds** (minimum effective **5**; default **30** if unset or too low). |
| `upstream_health_check_query_name` | QNAME for probes (default **`google.com.`** if empty). |

After you change these values in `dnsplane.json`, restart the dnsplane process so the new settings take effect.

## Logs

When an upstream transitions to **unhealthy**, a **warn** line is written to the DNS server log, e.g.
`upstream marked unhealthy after repeated probe failures` with `server` and `error`.

## REST API

Replace `8080` with your `apiport`.

**Health-only JSON**

```bash
curl -sS http://127.0.0.1:8080/dns/upstreams/health | jq .
```

**Servers list + health**

```bash
curl -sS http://127.0.0.1:8080/dns/servers | jq '{enabled: .upstream_health_check_enabled, health: .upstream_health}'
```

Response fields per upstream (`upstreams` / `upstream_health`):

- `address_port` — e.g. `8.8.8.8:53`
- `unhealthy` — `true` if excluded from forwarding
- `consecutive_failures` — current failure streak from probes
- `last_probe_at`, `last_probe_error` — last probe outcome
- `last_success_at` — last probe or successful forward

When checks are **disabled**, `unhealthy` is always `false` in the API (filtering is off).
