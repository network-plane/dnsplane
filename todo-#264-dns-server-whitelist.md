# Todo #264: DNS server domain whitelist (per-server, domain-specific upstreams)

## Goal

Support DNS servers that are used **only for specific domains**. For example:

- Add server `192.168.5.5` with a whitelist containing `internal.vodafoneinnovus.com`.
- Any query whose name matches that whitelist (e.g. `api.internal.vodafoneinnovus.com`, `internal.vodafoneinnovus.com`) is sent **only** to that server (and not to other upstreams).
- Queries for other domains are **not** sent to that server; they use the existing “global” upstreams and fallback as today.

So: **whitelist = “this server only receives queries for these domains, and those domains are only resolved via this server”.**

---

## Current behavior (brief) — *updated after implementation*

- **`dnsservers.DNSServer`**: has optional `DomainWhitelist []string`. Empty/nil = global server.
- **`dnsservers.GetServersForQuery(servers, queryName, activeOnly)`**: returns servers for this query (whitelist match or global). `GetDNSArray` still used where “all active” is needed.
- **Resolver**: uses `GetServersForQuery` in `resolveAParallel` (A/AAAA) and `handleDNSServers` (other types). Fallback only when not using whitelist selection.
- **CLI/TUI**: `dns add|update` accept named params including `whitelist:suffix1,suffix2`. `dns list` shows Whitelist column.
- **tools dig**: uses `GetServersForQuery` when no `@server` is given.
- **API**: still no DNS server API (optional, see §7).

---

## Feasibility

**Feasible.** The change is localized to:

1. **Data model**: add an optional whitelist to `DNSServer` and persist it.
2. **Server selection**: instead of “all active servers” for every query, compute “servers that apply to this question name” (whitelist match) and “global servers” (no whitelist). If any whitelist matches, use only those servers for that query; otherwise use current behavior.
3. **Matching**: reuse “domain/suffix” semantics similar to adblock (exact or “query name ends with whitelist entry”) so that e.g. `internal.vodafoneinnovus.com` matches `api.internal.vodafoneinnovus.com` and `internal.vodafoneinnovus.com`.

No change to the upstream client, cache, or local records; only to **which** server list is used for a given `question.Name`.

---

## Design decisions to fix early

1. **Whitelist format**
   - **Option A**: One or more domain suffixes, e.g. `["internal.vodafoneinnovus.com"]`. Match: query name equals or ends with `.<entry>` (normalize trailing dot).
   - **Option B**: Substring: “query contains this string” (e.g. “internal.vodafoneinnovus.com” anywhere). Simpler but can over-match.
   - **Recommendation**: Option A (suffix / “domain and subdomains”), consistent with typical split-DNS and adblock-style matching.

2. **Overlap**
   - If two servers both whitelist overlapping domains (e.g. same suffix), either: (a) send the query to both and keep current “first authoritative / first success” behavior, or (b) define priority (e.g. longest match wins). Recommendation: (a) for v1 to avoid extra complexity.

3. **Fallback**
   - For a query that matches a whitelist server: use only that server (and no global upstreams). If that server fails, optionally use fallback or return failure. Recommendation: for whitelist-only queries, do **not** add the global fallback (strict “only this server”); optionally add a per-server fallback later.

4. **Backward compatibility**
   - Servers without a whitelist (or empty whitelist) behave as today: they are “global” and get all queries. So existing config and JSON stay valid if we add an optional `domain_whitelist` (or `whitelist`) field.

---

## Todo (implementation order)

### 1. Data model and persistence

- [x] **1.1** Add optional `DomainWhitelist []string` (or `Whitelist []string`) to `dnsservers.DNSServer` in `dnsservers/dnsservers.go`. Empty/nil = “global” server (current behavior).
- [x] **1.2** Ensure `data.LoadDNSServers` / `SaveDNSServers` and default JSON still work (optional field, backward compatible).
- [x] **1.3** If needed, add a small migration or default so existing `dnsservers.json` without the field still unmarshals (Go zero value is fine for `nil`/empty slice).

### 2. Domain matching

- [x] **2.1** Add a function in `dnsservers` (or a small shared package) to decide if a query name matches a server’s whitelist: e.g. `ServerMatchesQuery(server DNSServer, queryName string) bool`. Normalize `queryName` (lowercase, strip trailing dot). Match if any whitelist entry is exact or is a suffix of the query (e.g. query `api.internal.vodafoneinnovus.com` and entry `internal.vodafoneinnovus.com` → true). Reuse pattern similar to adblock’s `IsBlocked` (suffix / subdomain logic).
- [x] **2.2** Add a function to get “servers that apply to this query”: e.g. `GetServersForQuery(servers []DNSServer, queryName string, activeOnly bool) []string`. If any server has a non-empty whitelist and matches the query, return only those matching servers (address:port). Otherwise return “global” servers (no whitelist or empty whitelist). Clarify: “global” = servers that have no whitelist; they receive all queries that are not claimed by a whitelist server.

### 3. Resolver integration

- [x] **3.1** In `resolver/resolver.go`, replace the two places that call `GetDNSArray(store.GetServers(), true)` with logic that:
  - Takes `question.Name` and `store.GetServers()`.
  - Uses the new “get servers for this query” helper to obtain the list of upstream addresses to use for this question.
  - If the list is empty (e.g. query matches a whitelist but no server is active), optionally try fallback or return NXDOMAIN/no response (decide and document).
- [x] **3.2** Apply this in both `resolveAParallel` (A/AAAA) and `handleDNSServers` (other types). Fallback: only add fallback when the selected server list is “global” (or explicitly allow fallback for whitelist misses; recommend not using fallback for whitelist-only selection so that “only this server” is strict).
- [x] **3.3** Ensure caching and logging still work; they are per-question and don’t depend on which server list was used.

### 4. CLI/TUI (dns add/update/list)

- [x] **4.1** Extend `applyArgsToDNSServer` in `dnsservers/dnsservers.go` to accept an optional whitelist (e.g. extra args after the existing booleans, or a dedicated flag/keyword like `whitelist:suffix1,suffix2`). Define a clear syntax so it’s parseable and document it.
- [x] **4.2** Update `dns add` and `dns update` usage/help and parsing in commandhandler so users can add/update a server with a whitelist (e.g. `dns add 192.168.5.5 53 true true false whitelist:internal.vodafoneinnovus.com` or similar).
- [x] **4.3** Update `dns list` and `renderDNSServerTable` to show whitelist (e.g. extra column or a “Domains” column with comma-separated suffixes). Ensure TUI table still fits small screens (truncate or tooltip if needed).
- [x] **4.4** Add tests or manual checks for add/update/list with and without whitelist.

### 5. tools dig

- [x] **5.1** When no `@server` is given, tools dig currently uses `GetDNSArray(dnsData.GetServers(), true)`. Change to “servers for this query” using the same helper as the resolver, so that `tools dig api.internal.vodafoneinnovus.com` only queries the whitelisted server(s) for that name. When user specifies `@server`, keep current behavior (query only that server).

### 6. Documentation and tests

- [x] **6.1** Update README or docs: describe per-server domain whitelist, example (e.g. 192.168.5.5 for `*internal.vodafoneinnovus.com`), and that whitelist servers are exclusive for those domains.
- [x] **6.2** Add unit tests for: (1) domain matching (suffix / exact), (2) `GetServersForQuery` (whitelist match returns only matching servers; no match returns global only; empty whitelist = global).
- [ ] **6.3** Optional: integration test or manual test: one global server, one whitelist server; query whitelisted domain → only whitelist server used; query other domain → only global server used.

### 7. Optional / later

- [ ] **7.1** REST API for DNS servers (list/add/update/remove): if added, include `domain_whitelist` in request/response.
- [ ] **7.2** Per-server fallback when a whitelist server fails (e.g. optional second address for same whitelist).
- [ ] **7.3** Conflict resolution when multiple servers match the same query (e.g. longest suffix wins) if product requirements demand it.

---

## File touch list (for reference)

| Area              | File(s) |
|-------------------|--------|
| Data model        | `dnsservers/dnsservers.go` |
| Persistence       | `data/data.go` (unchanged if JSON field is optional); default template in `data` if we create `dnsservers.json` with new field. |
| Matching + selection | `dnsservers/dnsservers.go` (new helpers) or new `dnsservers/whitelist.go` |
| Resolver          | `resolver/resolver.go` (replace GetDNSArray with query-aware server selection in 2 places) |
| TUI add/update/list | `commandhandler/commandhandler.go` (dns add/update args, renderDNSServerTable) |
| tools dig         | `commandhandler/commandhandler.go` (runToolsDig server list) |
| Docs              | `README.md` or `docs/` |
| Tests             | `dnsservers/*_test.go`, optionally `resolver/*_test.go` |

---

## Summary

- **Feasibility**: High. No new infra; optional field + new “servers for query” logic and resolver call-site changes.
- **Risk**: Low if we keep whitelist optional and default to current behavior for servers without whitelist. Main risk is subtle bugs in “who gets the query” (e.g. fallback leaking into whitelist path); tests and a clear decision on fallback avoid that.
- **Effort**: Roughly small–medium: data + matching + resolver + CLI/TUI + tools dig + tests and docs.
