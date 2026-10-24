# State database & session storage — Index

Sub-plans (execution order):

- [ ] 01-foundation — Go module, deps (`glebarez/go-sqlite`, `pressly/goose/v3`), domain types + port interface, embedded goose migration (5 tables, indexes, WAL + `busy_timeout`), SQLite adapter boot with session locker, in-memory/temp-file test harness, dual-runner migration race test
- [ ] 02-writes — Aggregate-store write methods per ownership matrix: MCP-owned (create session, insert round payload, insert questions, close on legend round) and daemon-owned (answer autosave, comment add/delete, round submit)
- [ ] 03-reads-recovery — Load-aggregate by session id (single indexed read, no post-load merge), gap-based `IsSessionConsistent`, `updated_at` staleness fields, `daemon.json` marker file (write-on-bind + read helpers), concurrent-writer race + full-session roundtrip acceptance tests

Concurrent-eligible batches: `[01-foundation], [02-writes], [03-reads-recovery]`

Why this order and split: 01 defines the interface, schema, and harness everything imports; 02 and 03 both sit on 01's surface and share store internals, so they run sequentially — 02 before 03 because the load-aggregate and acceptance tests in 03 need real rows to read, which only 02's write methods produce. Each batch is the safe unit for one concurrent session; same-file edits stay within a batch.

Normative source: `docs/issues/1.md`. Decision trail: `docs/drafts/state.md`.
