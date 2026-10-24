> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

# 04 — Contract and Issue Docs

**Goal:** Write the contract document and update the issue set so the domain code and the written contract say the same thing.

**Architecture:** Documentation only — no code in this sub-plan. `docs/drafts/contract.md` is the single contract spanning wire and domain halves, citing `docs/drafts/initial_mockup/rounds.md` as the authoritative decision log. The rewritten `docs/issues/3.md` points to the contract and the code instead of embedding JSON examples. New `docs/issues/19.md` covers the state.db payload-version validator (schema version const + migration policy). Dependent issues (1, 4, 5, 8, 10, 11, 13, 15) are NOT mass-edited — per the grill decision, they adopt the domain during their own grilling; only their direct contradictions get a pointer note.

**Tech Stack:** Markdown files.

---

### Task 1: Create docs/drafts/contract.md

**Files:**
- Create: `docs/drafts/contract.md`

- [ ] **Step 1: Write the document**

```markdown
# Round/answer contract (issue 3)

Source of truth: the Go code in `internal/domain/round` and
`internal/domain/session`. This document explains the contract in prose;
`docs/drafts/initial_mockup/rounds.md` is the authoritative decision log the
final grill round (issue 3 grilling) amended in two places, noted below.

The service: an agent drives a grill session over MCP; the browser screens
render from server state; the agent sends only the current round and receives
the submitted answers.

## Domain model

### Rounds

- `round.Round` interface, two concrete kinds (`round.Kind`):
  `QuestionRound` (`KindQuestions`) and `LegendRound` (`KindLegend`).
  There is no single round struct with optional phase fields — illegal
  phase/field mixtures are unrepresentable.
- Both carry `SessionID` so payloads are self-describing;
  `grill_round(session_id, round)` keeps the id as a tool parameter too,
  and the store verifies they agree.
- `QuestionRound` fields: `SessionID`, `Number` (>= 1), `Intro` (markdown),
  `Recap []Branch`, `Questions []Question`.
- `Round/Branch`: `Tag`, `Title`, `Prose` (markdown), `NewlySettled []Tag`,
  `Served int`, `Retained int`. Frontier remaining is numeric — the prose
  rendering derives from the counts, never the other way round. The agent
  recomputes the frontier from the submitted answers.
- `LegendRound` fields: `SessionID`, `Title`, `Branches []LegendBranch`,
  each with `Title` and tagged `Bullets` (`Tag`, `Decision` markdown).

### Questions and choices

- `Question` is prompt-only: `Title`, `Abstract []Markdown`, `Choices`,
  `Recommendation`, `Reminders`. Answers live in `AnswerSheet`, aligned
  1:1 by position with the round's questions — an unanswered question has
  no fake zero answer.
- Numbering: choices and recommendations are **1-based**; `0` is the
  shared sentinel (`ChoiceIndexOther` / `RecommendationNone`); `-1`
  (`ChoiceIndexNone`) means the user skipped.
- Choice text may carry inline markdown (bold, links); counts and the
  recommended marker are structural fields, never embedded in prose.
- `Tags` are plain strings, never markdown.
- `type Markdown string`: every renderable blob (intro, abstract, recap
  prose, option text, legend decisions, comments) is a `Markdown`, making
  the structural-vs-markdown boundary compiler-checked.

### Answers

- `round.Answer`: `ChoiceIndex ChoiceIndex`, `Comments []Markdown`.
- `Answer.String(choices)` renders for the agent's chat history: listed
  choice text plus comments joined by blank lines; "Other" renders the
  comments only; skipped renders empty.
- `round.AnswerSheet`: `SessionID`, `Number`, `Answers []Answer`.

## Invariants (enforced in the domain)

- Round: non-empty `SessionID`, `Number >= 1`, `KindQuestions` rounds have
  >= 1 question, `KindLegend` rounds have >= 1 branch.
- Question: >= 1 choice; recommendation absent (0) or inside 1..N.
- Answer: skip ⇒ `ChoiceIndexNone`; "Other" ⇒ `ChoiceIndexOther` AND at
  least one comment; listed choice ⇒ 1..N.
- Answer sheet: one answer per question, aligned.
- All violations are sentinel errors (`round.Err*`); per-question errors
  are wrapped with the 1-based position.
- The 5-question / 5-option caps and any product policy are enforced by
  the tool layer (issue 5), the layer closest to the agent — NOT by the
  domain.

### Deliberate amendment: no round cap

`initial_mockup/rounds.md` (Round 3) documented a cap of 5 rounds per
session. The issue-3 grilling dropped it: rounds keep arriving while the
frontier is unresolved, and **frontier closure is the only termination
condition** of a session. There is no `MaxRounds` anywhere in the code.

## Session

- `session.Session`: `ID`, `Name` (tab/heading, from `grill_start`),
  `Rounds []round.Round`, `Status`.
- Status: `Active | Closed` (`StatusActive` maps to state's `active`,
  `StatusClosed` to `closed`). The transition is one-way and guarded:
  `session.Close()` errors with `ErrAlreadyClosed` on a second close. The
  tool closes the session when the legend round's answers are stored.
- One daemon hosts all sessions; the daemon itself is stateless — the db
  (issue 1) is the real host of session state. pid/port are daemon /
  adapter concerns, never session fields.

## Wire format (adapter duty)

All JSON mapping lives in `internal/port/round/json` (built by the tool
issues): DTO structs carrying `json` tags, `From*/To*` mapping helpers,
and a payload `schema_version` field stamped from `round.SchemaVersion`
(constant 1). The domain never imports `encoding/json`. Payload shapes
follow the field lists above; earlier JSON examples in the old issue 3
are superseded by this document.

## Schema versioning

- `round.SchemaVersion = 1` stamps every wire payload via the adapter.
- The version validator over persisted payloads and the change policy are
  specified in `docs/issues/19.md`.

## Testing gate

Table-driven `*_test.go` for every exported validator and rendering
method; no mocks in domain tests (there is nothing to mock). Mocks
arrive with the adapters in later issues.
```

- [ ] **Step 2: Commit**

Commit sequence from `core-commands`, message:

```
docs: add round/answer contract

Domain model, invariants, wire duty and the two grill amendments
(no round cap, numeric frontier) in one contract document.
```

---

### Task 2: Rewrite docs/issues/3.md

**Files:**
- Modify: `docs/issues/3.md` (full rewrite)

- [ ] **Step 1: Replace the whole issue file with**

```markdown
# Round payload domain (JSON contract)

## Context

The `grill_round` tool sends only the current round's data to the server.
Earlier rounds live in state; screens render from state. The contract now
lives as Go domain code, not as embedded JSON examples: the domain is pure,
JSON is an adapter (`internal/port/round/json`, built with issue 5).

Referenced in grill rounds: R3 (Q3.3 drive_round sends current round only),
R4 (Q4.3 structured envelope + markdown inline boundary), plus the
issue-3 grilling session (schema confirmed via legend).

## Deliverables

- `internal/domain/round`: rounds (`QuestionRound` / `LegendRound` sum
  types), questions, choices (1-based, 0 = other/none, -1 = skip), answers
  and answer sheets, `Markdown` type making the structural/markdown
  boundary compiler-checked, `Validate()` invariants with sentinel errors,
  `SchemaVersion` const.
- `internal/domain/session`: the grill descriptor (`Active | Closed`,
  guarded `Close()`); no daemon/pid state — the db is the real host of
  session state, one stateless daemon hosts all sessions.
- `docs/drafts/contract.md`: the contract prose (wire + domain halves),
  citing `docs/drafts/initial_mockup/rounds.md` as the decision log.

## Amendments of earlier documents

- No round cap: frontier closure is the only termination (amends the
  mockup's documented 5-round cap). Question/option caps stay tool policy
  (issue 5).
- Legend and question rounds are separate types; `frontier_remaining` is
  numeric (`Served`/`Retained`); active/closed session states only.

## Acceptance criteria

- A `QuestionRound` can express every element of grill-me's round format
  (intro, recap branches with numeric frontier counts, question blocks with
  abstract, options, recommendation, reminders); a `LegendRound` expresses
  the per-branch tagged bullets.
- `AnswerSheet` carries enough information for the agent to recompute the
  frontier (choice, skip, other, comments per question).
- Every type's `Validate()` catches its own invariants; violations are
  sentinel errors (`errors.Is`-able).
- Domain packages import nothing beyond stdlib (`errors`, `fmt`,
  `strings`); `encoding/json` stays out of `internal/domain`.
- Table-driven tests cover every exported validator and `String()`
  method; `go test ./... && go vet ./...` passes.

## Dependencies

None as a design base — issues 1, 4, 5, 8, 10, 11, 13 and 15 depend on
this contract; `docs/issues/19.md` (payload version validator) extends it.
```

- [ ] **Step 2: Commit**

Commit sequence from `core-commands`, message:

```
docs(issues): rewrite issue 3 as domain-code contract

Issue 3 now delivers domain types and delegates contract prose
to docs/drafts/contract.md.
```

---

### Task 3: Create docs/issues/19.md

**Files:**
- Create: `docs/issues/19.md`

- [ ] **Step 1: Write the issue**

```markdown
# Payload version validator (state.db schema versions)

## Context

Round and result payloads are persisted in `state.db` (issue 1) as JSON.
The wire format lives in the DTO adapter (`internal/port/round/json`), and
every payload carries a top-level `schema_version` stamped from
`round.SchemaVersion` (currently 1). The domain stays unversioned; DTOs
evolve and can be migrated or rejected at read time.

This issue defines how the store (issue 1) validates the version it reads.

Referenced in grill rounds: R4 (Q4.3 left room for skill-side copy changes
without server schema churn) and the issue-3 grilling (schema version with
Go const; validator documented in a dedicated issue).

## Requirements

1. `round.SchemaVersion` (const 1) is the single source of version truth;
   DTO adapters stamp it into every persisted payload.
2. The store, when reading round payloads or answer sheets from `state.db`,
   validates the recorded `schema_version` against `round.SchemaVersion`.
   - Match ⇒ proceed.
   - Mismatch ⇒ the record is unreadable: the daemon's state is stale for
     the running binary. The store reports a version error; sessions with
     unreadable records surface as empty in listings.
3. Change policy: the first DTO shape change that breaks old stored
   payloads bumps `round.SchemaVersion` and the store treats mismatched
   records as wipe candidates. A manual wipe (`grill-chef stop` + state.db
   removal) refreshes the state; no automatic migration runs in v1.
4. The validator lives with the store adapter (issue 1), uses `errors.Is`
   with a sentinel for "version mismatch", and is table-driven tested.

## Acceptance criteria

- Persisted payloads record `schema_version` matching `round.SchemaVersion`.
- Reading a payload with a foreign version yields a `version mismatch`
  sentinel error instead of decoding garbage.
- A `SchemaVersion` bump is documented as a breaking change in the DTO
  adapter and requires a state wipe, not a migration.

## Dependencies

Issue #1 (state database, owns the read path), issue #3 (domain contract,
owns `SchemaVersion`).
```

- [ ] **Step 2: Commit**

Commit sequence from `core-commands`, message:

```
docs(issues): add payload version validator issue

New issue 19 defines state.db schema version checking and the
bump policy: mismatch is wipeable staleness, not migration.
```

---

### Task 4: Cross-reference notes in directly contradicted issues

**Files:**
- Modify: `docs/issues/1.md` (one note)
- Modify: `docs/issues/4.md` (one note)
- Modify: `docs/issues/5.md` (one note)
- Modify: `docs/issues/13.md` (one note)

- [ ] **Step 1: Add a note after `## Requirements` in `docs/issues/1.md`**

```markdown
> Note (issue 3 grilling): session statuses are the domain's
> `Active | Closed` (rendered as `active | closed` at the store edge).
> Daemon state (pid/port) is stored alongside, as adapter data — never
> as part of session semantics.
```

- [ ] **Step 2: Add a note after `## Requirements` in `docs/issues/4.md`**

```markdown
> Note (issue 3 grilling): the session starts `Active` (rendered as
> `active` at the store edge); the pure `session.Session` type and its
> two-state machine live in `internal/domain/session`.
```

- [ ] **Step 3: Add a note to `docs/issues/5.md`**

After the Requirements heading, insert:

```markdown
> Note (issue 3 grilling): answers carried by `grill_round` return as a
> `round.AnswerSheet` aligned with the questions (1-based choices, skip =
> `ChoiceIndexNone`, other = `ChoiceIndexOther` + >= 1 comment). Product
> caps (5 questions / 5 options) are enforced here, in the tool — not in
> the domain. There is no round cap: frontier closure terminates the
> session.
```

- [ ] **Step 4: Add a note to `docs/issues/13.md`**

After the Requirements heading, insert:

```markdown
> Note (issue 3 grilling): the session is closed via the domain's guarded
> `session.Close()` when the legend round's answers are stored — the same
> moment this issue's rendering marks it `closed` in state.
```

- [ ] **Step 5: Commit**

Commit sequence from `core-commands`, message:

```
docs(issues): note contract changes in directly hit issues

One annotation each in issues 1, 4, 5 and 13 so their next
grilling adopts the domain without mass edits.
```

---

### Task 5: Sub-plan boundary check

- [ ] **Step 1: Run the whole module's tests and go vet**

Run: `go test ./... && go vet ./...`
Expected: previous green state preserved; docs-only changes keep everything passing
