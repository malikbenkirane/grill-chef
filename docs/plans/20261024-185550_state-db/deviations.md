# Deviations — 20261024-185550_state-db

Recorded during execution of sub-plan 01-foundation on 2026-10-26 with
goose v3.29.0 (the plan was drafted against an earlier goose API draft).

## 01-foundation

1. **Dependencies appear in `go.mod` later than Task 1 specifies.**
   After `go mod init` + `go get` + `go mod tidy`, `tidy` removed both
   requirements because no package imported them yet (the module only
   contained `internal/domain`). Both deps (glebarez/go-sqlite v1.23.0,
   pressly/goose/v3 v3.29.0) were re-pinned as soon as the migration and
   adapter code imported them (Tasks 4–5). Final state matches the plan's
   expectation: exactly the two allowed external requirements plus
   goose's transitive graph, no cgo sqlite.

2. **Task 4 migration is an embedded SQL file, not a Go migration.**
   The plan's `migrations/00001_init.go` used `goose.AddMigrationContext`
   with `func(ctx, *sql.DB)` signatures. Goose v3.29 changed both the
   registration signature (Go migrations receive `*sql.Tx`) and, more
   importantly, how session locks interplay with the migration
   transaction (see 4). The migration now lives in
   `internal/port/store/sqlite/migrations/00001_init.sql` with
   `-- +goose NO TRANSACTION` and explicit `-- +goose Up / Down`
   sections; statement content is identical to the plan's DDL
   (six tables, indexes, CHECKs, PKs, FKs). The Go migrations package
   was deleted.

3. **The embed FS lives in `embed.go`, not `store.go`.** Task 4's tests
   need `migrationsFS` to compile before Task 5's store exists, so the
   `//go:embed migrations` var was split into
   `internal/port/store/sqlite/embed.go`.

4. **Migration locking is a single `BEGIN IMMEDIATE` (settled design kept),
   but requires `WithIsolateDDL(true)` and a no-transaction migration.**
   Goose v3.29 runs `ensureVersionTable` and each migration inside its own
   transaction *on the locked connection*. A `BEGIN IMMEDIATE` opened in
   `SessionLock` therefore nested against goose's own `BEGIN` and failed
   with "cannot start a transaction within a transaction". Fix: the SQL
   migration carries `NO TRANSACTION`, and the provider is built with
   `WithIsolateDDL(true)` so the version table and the DDL run directly on
   the locked connection — inside the locker's single `BEGIN IMMEDIATE`,
   committed in `SessionUnlock`. Cross-process arbitration, WAL +
   `busy_timeout`, and the "no retry loop in the locker" rule all still
   hold as settled. Cross-process detail: goose v3.29 re-reads applied
   versions *inside* the lock before applying, so the racing loser
   converges to zero applied migrations (Task 7 passes `count=5`).

5. **journal_mode(WAL) moved from the DSN to an explicit `ensureJournalMode`
   step with a bounded re-check loop.** With `journal_mode(WAL)` as a
   connection DSN pragma, opening a second process's first connection
   while the migrator holds its `BEGIN IMMEDIATE` failed fast with
   `SQLITE_BUSY` (journal-mode switching requires an exclusive lock and
   SQLite's busy handler does not cover it). WAL is persistent, so Boot
   now reads the current mode first (lock-free); if it is already `wal` it
   proceeds, otherwise it switches once. On the rare first-boot race where
   the other process is literally mid-migration, the switch retries on a
   100 ms check loop bounded by a 5 s deadline (mirroring `busy_timeout`)
   — this is the one retry loop the plan told the implementer not to
   resurrect *in the locker*; it lives in journal-mode setup instead,
   which the plan's DSN pragma implicitly assumed away.

6. **`goose.WithSessionLocker`'s interface shape changed.** The plan's
   locker implemented `Lock(ctx, *sql.DB)`/`Unlock` (goose ≤3.25 style).
   Goose v3.29 defines `lock.SessionLocker` as
   `SessionLock(ctx, *sql.Conn)` / `SessionUnlock(ctx, *sql.Conn)` — the
   provider acquires and owns the connection. The adapter's locker was
   reshaped accordingly; DST behavior (single `BEGIN IMMEDIATE` under the
   caller's context deadline, `COMMIT` on unlock with a detached context)
   is unchanged.

7. **Minor doc fix (Task 2 scope):** `session.Status`'s doc comment said
   "Two states only"; amended to mention `StatusDismissed` so the comment
   matches the iota the same commit adds.

8. **issues/1.md topology table:** besides the plan's three edits, the
   `legends` table was added as its own row (MCP-owned), since the
   six-table schema introduces a writer's surface the original table did
   not enumerate.

No deviations in domain semantics, port surface, schema shape, ownership
matrix, or test scope. Sub-plans 02 and 03 are untouched and their
harness (`newTestStore`, `newMemoryStore`) exists as specified.
