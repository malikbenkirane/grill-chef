# Draft — state database decisions (grill transcript, issue 1)

Reference for everything settled while grilling `docs/issues/1.md`. The issue
file carries the normative requirements; this document carries the *why* and the
conversation trail for each decision.

## Engine & dependencies

- **SQLite via `glebarez/go-sqlite`** (pure-Go driver, no cgo). The original
  `state.db` JSON layout and the stdlib-only clause in issue 1 were both
  discarded when the dual-writer topology made an engine mandatory.
- **`pressly/goose/v3` in library mode** for migrations: Go-code migrations
  with `init()` registration, embedded with `//go:embed`, applied idempotently
  at startup via `goose.NewProvider(goose.DialectSQLite3, db, fsys)` +
  `provider.Up(ctx)`. No CLI at runtime. Reference:
  ulearn TIL "goose v3 programmatic migrations" (provider API, v3.16+;
  the pre-2023 globals API was rejected as its config is package-level state
  and its concurrent calls are unsafe).

## Ownership & concurrency

The original single-writer decision (one daemon owning the file, everything
via HTTP) was **revised** once the process split became real: the MCP stdio
process and the web daemon are separate processes of the same binary, and
forcing all mutations through the daemon would have meant proxying agent
tool calls over HTTP only to bounce them back into process-local state.

- **MCP (stdio) process owns**: `sessions`, `questions`, round payload columns.
- **Web daemon owns**: `answers`, `comments`, and the daemon-owned
  `round.status` column (submit).
- Domains remain disjoint by *row kind*, not by session: an agent mid-round and
  a browser autosave against the same session touch different tables, so
  writer contention serializes at the database level and never mutates
  another writer's rows. WAL + `busy_timeout` in the setup DDL handle the
  rest (submit racing legend `grill_round` hits a different table/row).
- The ownership contract is **documented per-table in issue 1 and enforced at
  the type level** through the store's aggregate methods, not by convention.

## Schema

- Five tables: `sessions`, `rounds`, `questions`, `answers`, `comments`.
  Comments are first-class (own rows) because the DELETE-per-comment route in
  issue 8 makes them independently mutable; answers are separate from question
  payloads because the autosave path is per-question and hot.
- FK spine on `session_id`, composite indexes on `(session_id, round)` and
  `(session_id, round, question)`. Full rehydration is one indexed read; no
  post-load merge in Go.
- Session statuses: `active` / `closed` / `dismissed` — the third value was
  added because the store home's dismiss action is a status change, never a
  delete. Retention policy: keep everything.

## Lifecycle & recovery

- **`closed` fires when a legend `grill_round` round payload is processed**, not
  when all rounds report submitted. The issue text originally leaned on
  round-count; rounds.md instead ties closing to the final legend payload.
  (The shared-understanding confirmation still lives in chat, per grill-me.)
- **Daemon pid+port live in a marker file` `daemon.json` next to the db,
  rewritten on bind.** Tried as a sqlite table first (initial issue text) and
  rejected: recovery tooling and the human-only `stop` command would then have
  to open (and checkpoint) a possibly-hot WAL just to learn whether the daemon
  is alive, and a stale row can point at a reassigned port. The fs is the
  right place for facts about the fs's neighbor process.
- **Consistency check is gap-based**, not timestamp-based: round numbers are
  contiguous 1..N with no voids, every pending round carries its full question
  set. Timestamps (`updated_at`) are kept as a separate staleness commodity —
  they detect "sat idle for days", not "write aborted mid-round".

## Migration & code surface

- Both processes run migrations at boot with `goose.WithSessionLocker(...)`,
  which makes start order irrelevant (daemon respawned in any order, or racing
  the MCP process, all converge).
- The store's Go surface is a **domain aggregate**, one method per write path,
  so the ownership matrix is enforced at the type level. Thin typed CRUD was
  rejected because it lets the wrong process write rows it shouldn't, with no
  compile-time signal.
- **Hexagonal pattern: `internal/port/store` (interface) + SQLite adapter
  under `internal/port/store/sqlite`.** Domain layer never touches SQL.
  Extra adapters (postgres, in-memory for tests?) left open and deliberately
  unanticipated until a need appears.

## Rejected alternatives (for future archaeology)

- Single JSON `state.db` rewritten per mutation — write amplification grows
  with session count; one truncated write could take down every session.
- Directory-of-per-session-JSON-files — cheap O(1) writes, but two files to
  keep in sync on create and a hand-rolled index.
- Multi-writer through *unrestricted* sqlite connections — was floated
  mid-grill; the settled compromise keeps the writers to exactly two
  processes with disjoint row ownership plus WAL + `busy_timeout`.
- Single-writer daemon-owns-everything (original Q1.1 recommendation) —
  contradicted by the real MCP/daemon process split.
- Daemon pid/port inside a sqlite table (see Lifecycle section above).
- goose deferred / hand-rolled `CREATE TABLE IF NOT EXISTS` — rejected in
  favor of explicit tracked migrations, on the basis that a schema-only
  change arrives with the next schema need anyway.
