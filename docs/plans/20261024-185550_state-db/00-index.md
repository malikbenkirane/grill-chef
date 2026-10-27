# State database & session storage — Index

Sub-plans (execution order):

- [x] 01-foundation — Go module, deps (`glebarez/go-sqlite`, `pressly/goose/v3`), domain upgrades (`StatusDismissed`, `UpdatedAt` staleness fields), domain-typed port interface (two round-create methods, `DismissSession`), embedded goose migration (six tables: sessions, rounds, questions, answers, comments, legends; no round-cap CHECK; per-question comment seq PK), SQLite adapter boot with WAL + `busy_timeout` and a single-`BEGIN IMMEDIATE` session locker, in-memory/temp-file test harness, dual-runner migration race test, plus the issue-1/state.md doc reconciliation (six tables, dismissed domain-owned)
- [x] 02-writes — Ownership-matrix write paths with TDD: adapter payload DTOs + column mappers; MCP-owned (CreateSession, CloseSession, CreateQuestionRound, CreateLegendRound); daemon-owned (UpsertAnswer with domain sentinels, SubmitRound idempotent, DismissSession, AddComment/DeleteComment with per-question seq); MCP↔daemon write race test
- [x] 03-reads-recovery — Aligned rehydrated answers on the domain question round, single indexed read `LoadSession`, gap-based `IsSessionConsistent` (payload `question_count` contract), staleness gate, `daemon.json` marker file (atomic write/read helpers), full-session roundtrip + multi-session concurrency acceptance tests

Concurrent-eligible batches: `[01-foundation], [02-writes], [03-reads-recovery]`

Why this order and split: 01 defines the interface, schema, and harness everything imports; 02 and 03 both sit on 01's surface and share store internals, so they run sequentially — 02 before 03 because the load-aggregate and acceptance tests in 03 need real rows to read, which only 02's write methods produce. Each batch is the safe unit for one concurrent session; same-file edits stay within a batch.

Normative source: `docs/issues/1.md`. Decision trail: `docs/drafts/state.md`. Grilling settlement (2026-10-25): port references `internal/domain` exclusively; legend rounds live in the `legends` relation; persistence stores domain conventions verbatim (1-based, sentinels); comments drop AUTOINCREMENT for `(session_id, round_number, question_index, comment_seq)`; the migration locker is a single `BEGIN IMMEDIATE` under the caller's context deadline.
