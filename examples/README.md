# Example files

| File | Purpose |
| --- | --- |
| [dnsplane-example.json](dnsplane-example.json) | Starter `dnsplane.json` (DoT / DoH / DNSSEC keys present, off by default). Includes **`dashboard_resolution_log_cap`** (default `1000`) for the in-memory dashboard **Resolutions log**. |
| [dnsservers-example.json](dnsservers-example.json) | Example `dnsservers.json` with a global upstream and **`domain_whitelist`**. |
| [curl-stats-dashboard.sh](curl-stats-dashboard.sh) | **`curl`** calls for `GET /stats/dashboard`, `/stats/dashboard/data`, **`/stats/dashboard/resolutions`**, and **`POST /stats/dashboard/resolutions/purge`**. Set `BASE` (default `http://127.0.0.1:8080`). Add `Authorization: Bearer …` if `api_auth_token` is set. |

The same config shape as `dnsplane-example.json` (including `dashboard_resolution_log_cap`) lives at **[../docs/dnsplane.example.json](../docs/dnsplane.example.json)** for the full path referenced from the main README.
