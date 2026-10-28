# State database & session storage

**Issue**: 1
**Plan:** `docs/plans/20261024-185550_state-db` (00-index, 01-foundation, 02-writes, 03-reads-recovery)
**Base:** `rkvnmmmu docs(plans): reconcile state-db index with grilling settlement`
**Head:** `tqnwsmqq docs(plans): close out state-db plan, record 02/03 deviations`
**Normative sources:** `docs/issues/1.md`, `docs/drafts/state.md`
**Deviations from plan:** `docs/plans/20261024-185550_state-db/deviations.md` (14 recorded)

## Summary

Makes the grill-chef MCP server crash-proof by persisting all session state to a single-user SQLite file, so any grill-chef process can rehydrate any session from its id alone. The store is a hexagonal port typed in `internal/domain` aggregates, implemented by a pure-Go SQLite adapter under `internal/port/store/sqlite`, with an ownership matrix splitting writes between the MCP (agent) process and the web daemon. Full schema and boot conventions match the grilling settlement recorded in the plan index (2026-10-25).

## What changed

### 01 — Foundation

- `go.mod`: exactly two external dependencies — `glebarez/go-sqlite` (pure Go, no cgo; preserves the single-binary promise) and `pressly/goose/v3` (library mode).
- **Domain upgrades** (`internal/domain/{session,round}`):
  - `session.StatusDismissed` — the domain is the state's truth; station-home dismiss (issue 14) is a status change, never a delete.
  - `UpdatedAt` staleness fields on `QuestionRound`, `LegendRound`, and `Session` — populated by rehydration, never consulted for correctness.
- **Port** (`internal/port/store/port.go`): every method belongs to one writer process. MCP-owned: `CreateSession`, `CloseSession`, `CreateQuestionRound`, `CreateLegendRound`. Daemon-owned: `UpsertAnswer`, `SubmitRound`, `AddComment`, `DeleteComment`, `DismissSession`. Shared readers: `ListSessions`, `LoadSession`, `IsSessionConsistent`. Persistence stores domain conventions verbatim (1-based choice indexes, sentinels `-1` none / `0` other).
- **Schema** (embedded goose migration, six tables): `sessions`, `rounds`, `questions`, `answers`, `comments`, `legends`. Legend rounds live in their own relation, never in `rounds`. Comments drop AUTOINCREMENT for PK `(session_id, round_number, question_index, comment_seq)` — the stable identity issue 8's DELETE route carries. No round-cap CHECK at the store layer; contiguity and legend-at-end are domain policy.
- **Adapter boot**: `file:` DSN with `_pragma` keys (`busy_timeout(5000)`, `foreign_keys(1)`), idempotent migration under a single `BEGIN IMMEDIATE` session locker (context deadline governs; no retry loop), WAL re-asserted idempotently at boot, `Ready()` health probe.
- **Race-tested boot**: two processes booting the same file concurrently both converge on the migrated schema (`-race -count 5` clean).
- **Docs reconciliation**: issue 1 and drafts/state.md updated to six tables, dismissed status, legend relation, per-question comment identity.

### 02 — Writes

- Payload DTOs + column mappers: `rounds.payload` holds intro + recap + `question_count` only; `legends.payload` holds the whole `LegendRound`; question bodies serialize to their own `questions` rows (no post-load merge).
- MCP session writes: `CreateSession` persists the aggregate verbatim (rejects undefined status), `CloseSession` flips active→closed exactly once.
- MCP round writes: one transaction writes the rounds row plus one questions row per question; legend round lands in `legends`, duplicate on PK fails.
- Daemon writes: `UpsertAnswer` autosave hot path stores domain sentinels verbatim; idempotent `SubmitRound` (unknown rounds still fail); `DismissSession` flips status without deleting anything; `AddComment` allocates `MAX(comment_seq)+1` per question in one transaction so a deleted seq is never reused; `DeleteComment` removes exactly one row by identity.
- Race acceptance: concurrent MCP round-create and daemon autosave never corrupt (`-race -count 5` clean).

### 03 — Reads & recovery

- `QuestionRound.Answers`: positionally aligned rehydrated answers (1..len(Questions), unanswered = `ChoiceIndexNone`), comments attached to their question's answer.
- `LoadSession`: whole aggregate rehydrated in one read-only transaction — sessions row, round payloads, question/answer/comment rows, legend row. Single indexed read set; no post-load merge.
- `IsSessionConsistent`: gap-based (contiguous 1..N over question rounds via DISTINCT-count == MAX probe) plus payload `question_count` contract for every pending round; unknown session is an error, not a false negative. Consumed by `grill_recover` (issue 4). Staleness fields populate but are never part of the check.
- `internal/port/daemoninfo`: atomic (tmp+rename) `daemon.json` pid/port marker; lives next to the db, read by recovery tooling and the human `stop` CLI without touching the db or its WAL sidecars.
- Acceptance: full grill lifecycle (2 rounds → answers → comment → submit → legend → close) roundtrips losslessly; six concurrent multi-session writers produce consistent, rehydratable sessions (`-race -count 3` clean).

## Key engineering notes

1. **goose v3.29 API migration.** Goose changed both the Go-migration signature (`*sql.Tx`) and `SessionLocker` (`SessionLock/SessionUnlock` on a provider-owned `*sql.Conn`). A Go migration cannot run under the locker's `BEGIN IMMEDIATE` (nested transaction). Solution: the migration is an embedded `-- +goose NO TRANSACTION` SQL file and the provider is built with `WithIsolateDDL(true)` — the version table and DDL execute directly on the locked connection, inside the single `BEGIN IMMEDIATE`, committed on unlock. Cross-process arbitration, one-transaction semantics, and the "no retry loop in the locker" rule all hold as settled.
2. **Racing inner migration applies are safe.** Goose re-reads applied versions *inside* the lock before applying, so the racing loser converges to zero applied migrations (verified `count=5`).
3. **`journal_mode(WAL)` left the DSN.** Opening a second process's first connection while the migrator holds its write tx fails fast with `SQLITE_BUSY` (journal-mode switch needs an exclusive lock; the busy handler does not cover it). WAL is persistent, so boot reads the mode first (lock-free no-op when already `wal`) and switches once otherwise, with a bounded 5 s re-check for the rare first-boot race. Recorded as deviation 5.
4. **`session.Session.UpdatedAt`.** Specified by 01's design notes but omitted by its Task 2 snippet; added with 03's LoadSession (deviation 11).

## Verification

- `go build ./... && go vet ./...` — clean.
- `go test ./... -race` — `ok` for `domain/round`, `domain/session`, `port/daemoninfo`, `port/store/sqlite`.
- `CGO_ENABLED=0 go build ./...` — cgo-free single binary; only `glebarez/go-sqlite` + `modernc.org` transitives in the graph (no `mattn/go-sqlite3`).
- Targeted: migration idempotence tests; dual-runner boot race (`-race -count 5`); MCP↔daemon write race (`-race -count 5`); acceptance roundtrip + multi-session concurrency (`-race -count 3`).

## Commit trail (sequential)

| Change | Subject |
|---|---|
| rkvnmmmu | (base) docs(plans): reconcile state-db index with grilling settlement |
| ymysuwtn | feat(store): init module with pure-Go sqlite and goose deps |
| pwozrssy | feat(domain): dismissed status and staleness timestamps |
| zqzlsmym | feat(store): domain-typed port interface encoding ownership matrix |
| mxnsvvlq | feat(store): goose migration for six-table session schema |
| knynulvl | feat(store): sqlite adapter boot with WAL, busy_timeout, locker migration |
| kpoxvnrz | test(store): shared temp-file and in-memory sqlite harness |
| ktlmtysq | test(store): two processes race on same store both migrate safely |
| ynzovtmz | docs(issue-1): six-table schema, dismissed status, daemon dismiss write |
| 6e9547d1 | docs(plans): record state-db deviations, check off 01-foundation |
| norkyrpm | feat(store): adapter payload dtos and column mappers |
| yotooxvo | feat(store): mcp session create and close writes |
| plwpwztw | feat(store): mcp question and legend round writes |
| ssowqyrq | feat(store): daemon answer autosave upsert with domain sentinels |
| tkxrsuvw | feat(store): daemon round submit and station-home dismiss writes |
| nxtmprmo | feat(store): daemon comment add and delete with stable per-question seq |
| uwowurmz | test(store): mcp and daemon writes race without corruption |
| syprkpqx | feat(domain): aligned rehydrated answers on question round |
| xkzkwnrk | feat(store): single-read session rehydration with aligned answers |
| tkqmzsmw | feat(store): gap-based session consistency check |
| szporqos | test(store): staleness fields populate through rehydration |
| vuxwxqmv | feat(store): daemon pid/port marker file helpers |
| mmxmsmks | test(store): full session roundtrip and multi-session concurrency acceptance |
| tqnwsmqq | docs(plans): close out state-db plan, record 02/03 deviations |

## Acceptance criteria (issue 1)

- [x] Can create a session, store rounds, store answers, and retrieve the full session state by id (`TestFullSessionRoundtrip`).
- [x] Daemon metadata (pid + port) persists across process restarts (`internal/port/daemoninfo`).
- [x] Concurrent writes to different sessions do not corrupt (`TestConcurrentMultisessionWritesDoNotCorrupt`, `TestMCPAndDaemonWritesRaceSafely`).
- [x] A stale session (no submitted answers after hours/days) is fully recoverable from its id alone (`TestLoadSessionRehydrates`, `TestStalenessFieldsPopulate`, `IsSessionConsistent`).
