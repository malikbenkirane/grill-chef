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
  specified in `docs/issues/20.md`.

## Testing gate

Table-driven `*_test.go` for every exported validator and rendering
method; no mocks in domain tests (there is nothing to mock). Mocks
arrive with the adapters in later issues.
