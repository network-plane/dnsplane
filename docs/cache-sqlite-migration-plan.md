# Plan: Resolver cache — JSON (`dnscache.json`) → SQLite

This document is a phased plan to move durable cache storage from a single JSON file to SQLite while preserving behavior (lookup semantics, TUI/API, compaction, async persist, config).

## Goals

- **Primary:** Replace `dnscache.json` with a SQLite database file as the on-disk cache store.
- **Preserve:** Same logical cache behavior (TTL/expiry, min TTL, RR-set synthetic entries, stale-while-revalidate flows, cluster-unrelated local cache).
- **Non-goals (initially):** Changing the hot-path architecture from “in-memory slice + index” unless a later phase explicitly targets it.

## Recommended approach: phased

### Phase A — SQLite as persistence backend (low risk)

Keep **`[]dnsrecordcache.CacheRecord` + `cacheRecordIdx`** as today. Only change **load at startup** and **save on persist**:

| Today | After Phase A |
|--------|----------------|
| `LoadCacheRecords()` reads JSON | Read all rows from SQLite into `[]CacheRecord`, rebuild index |
| `SaveCacheRecords()` writes full JSON | Transaction: replace table contents or `DELETE` + batch `INSERT` from in-memory snapshot |
| `cachePersistWorker` snapshots slice → JSON | Same worker; snapshot → SQLite write |

**Why first:** Resolver (`GetCacheRecords` / `UpdateCacheRecords`), `lookup_index.go`, compaction, and TUI/API stay structurally the same. Risk is confined to I/O, schema, and migration tooling.

### Phase B (optional) — Incremental SQL writes

Reduce work on each cache update:

- Persist **deltas** (upsert/delete by key) instead of full table rewrite, or
- Debounced “flush dirty keys” batch.

Requires defining a **stable primary key** (e.g. normalized `name` + `type` + hash or value prefix for RR-set rows) and tracking dirty state.

### Phase C (optional) — Query-backed store

Replace full-slice get/set with a **`CacheStore` interface** (get by name/type, upsert, delete expired, iterate for list). Touches `resolver.Store`, `resolver/resolver.go` cache paths, and `data/lookup_index.go`. Highest effort and regression risk; only if Phase A proves insufficient (e.g. very large caches, memory pressure).

**This plan focuses on delivering Phase A completely; Phases B/C are follow-ups.**

---

## Technical design (Phase A)

### Dependency

- Prefer **`modernc.org/sqlite`** (pure Go, no CGO) unless project standard mandates `github.com/mattn/go-sqlite3`.
- Add `database/sql` usage in a small package (e.g. `dnscachedb` or `data/dnscache_sqlite.go`).

### Schema (sketch)

One row per cache entry; store full `DNSRecord` + cache metadata without fighting column explosion:

```sql
CREATE TABLE cache_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name_norm TEXT NOT NULL,      -- normalized lookup key (same rules as today)
  rrtype TEXT NOT NULL,
  value TEXT NOT NULL,           -- includes RR-set prefix strings where used
  ttl INTEGER NOT NULL,
  expiry_unix INTEGER NOT NULL, -- or TEXT RFC3339; INTEGER is fine for queries
  timestamp_unix INTEGER,
  last_query_unix INTEGER,
  record_json TEXT NOT NULL      -- JSON of dnsrecords.DNSRecord (or full CacheRecord) for fields not in columns
);
CREATE INDEX idx_cache_expiry ON cache_entries(expiry_unix);
CREATE INDEX idx_cache_lookup ON cache_entries(name_norm, rrtype);
```

Adjust columns after auditing `dnsrecords.DNSRecord` JSON tags and what `dnsrecordcache` / resolver rely on.

**Versioning:** `PRAGMA user_version` or a `schema_meta` table for migrations.

### API surface (internal)

- `Open(path string) (*sql.DB, error)` — `WAL` mode, `busy_timeout`, single writer discipline aligned with current `persistCh` worker.
- `LoadAll() ([]dnsrecordcache.CacheRecord, error)`
- `ReplaceAll(records []dnsrecordcache.CacheRecord) error` — single transaction; used by persist worker (and optionally `cache save`).
- `Close() error`

### Config / paths

- Reuse **`file_locations.cache`** (or rename in docs to “cache store path”): value becomes e.g. `dnscache.db` instead of `dnscache.json`.
- **Defaults:** new installs get `dnscache.db`; document upgrade path for existing `dnscache.json`.
- **`pretty_json`:** does not apply to SQLite (document; no behavior change for other JSON files).

### Startup / shutdown

- **Initialize:** If path is SQLite (by extension or explicit mode), open DB, run migrations, `LoadAll` → `d.CacheRecords` + `rebuildCacheIndexLocked`.
- **Default file creation:** replace JSON template with empty DB + schema (or open creates schema on first connect).
- **Shutdown:** Existing `Close()` on `DNSResolverData` should close `*sql.DB` after final persist (same ordering as today).

### One-time migration

- On first start with `dnscache.db` missing but **`dnscache.json` present**: optional **auto-import** (flag or default once) or documented **`dnsplane` / TUI command** `cache import-json <path>`.
- Import: parse JSON → `[]CacheRecord` → `ReplaceAll` (or in-memory load then normal persist).

### Feature parity checklist

| Area | Action |
|------|--------|
| `data.Initialize` | Load from SQLite instead of JSON |
| `cachePersistWorker` | Snapshot → `ReplaceAll` |
| `CompactExpiredCacheRecords` | Unchanged in memory; persist still queued |
| `commandhandler` `cache load` / `cache save` | Read/write SQLite; update help strings |
| `cache clear` | Clear memory + persist empty table |
| `cache compact` | Unchanged logic; persist via worker |
| `api` cache flush | Same as clear + persist |
| `config` `cache_file` / CLI `--cache` | Document `.db` path |
| Tests | New tests for load/save round-trip, migration import, empty DB |

---

## Work breakdown (Phase A)

1. **Spike (0.5–1 d):** Pick driver, prove `Open` + `ReplaceAll` + `LoadAll` with 10k rows, measure time vs JSON.
2. **Package + schema (1–2 d):** `dnscachedb` (or under `data/`), migrations, indexes.
3. **Wire `data` (1–2 d):** Replace `LoadCacheRecords`/`SaveCacheRecords` call sites or branch on storage backend; integrate `Close()`; keep `persistCh` semantics.
4. **Config & UX (0.5–1 d):** Defaults, examples, README, TUI/API help text, `server set` descriptions.
5. **Migration tool / auto-import (0.5–1 d):** JSON → DB path.
6. **Tests (1–2 d):** Unit tests for DB layer; integration test server start with DB; regression on resolver cache tests.
7. **Review (0.5 d):** Locking, shutdown, corruption handling (log + fail safe).

**Total Phase A estimate:** ~1–2 weeks calendar time for one developer (includes review and fixes), depending on test depth and migration UX.

---

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| SQLite locked / writer contention | WAL + busy timeout; single writer via existing persist goroutine; avoid writes on DNS thread |
| Corrupt DB | Backup before replace; `ReplaceAll` in transaction; document `sqlite3` recovery for operators |
| Large DB memory | Phase A still loads full cache into RAM — same as JSON today; Phase B/C if that’s the real problem |
| Cluster | Cache is local; no cluster sync change expected — confirm no hidden JSON assumptions |

---

## Out of scope (unless added later)

- Syncing cache across cluster nodes
- Encrypting the SQLite file
- Changing `resolver.Store` contract (Phase C)
- Unifying with `fullstats` (bbolt) storage

---

## Success criteria

- All existing tests pass; new tests cover SQLite load/save and migration.
- Cold start with empty DB and with migrated data behaves like current JSON behavior for lookups and expiry.
- No regression in shutdown flush of cache to disk.
- Documentation lists new default filename and migration steps.

---

## Suggested commit / PR sequence

1. Add `dnscachedb` + tests (no wiring).
2. Wire load/save in `data` behind a small interface or file-extension switch.
3. Config defaults + docs + examples.
4. JSON import path + doc.
5. Remove or deprecate JSON write path (optional flag `cache_storage: json|sqlite` for one release if you want a soft transition).

---

## References in codebase (for implementers)

- `data/data.go` — `LoadCacheRecords`, `SaveCacheRecords`, `cachePersistWorker`, `storeCacheRecords`, `InitializeJSONFiles`
- `data/lookup_index.go` — cache index over slice
- `data/cache_compact.go` — expiry compaction
- `resolver/resolver.go` — `GetCacheRecords` / `UpdateCacheRecords` usage
- `commandhandler/commandhandler.go` — `cache` subcommands
- `api/server.go` — cache API
- `config/config.go` — `FileLocations.CacheFile`
