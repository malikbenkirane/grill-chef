# Round & answer domain (#3)

## Summary

Implements the pure DDD domain layer agreed in the issue-3 grill legend: the quiz model every later issue builds on, plus the contract prose so code and documents say the same thing.

## Changes

- **`internal/domain/round`** — the round vocabulary and invariants:
  - Strong `Markdown` and `Tag` types (structural-vs-markdown boundary is compiler-checked), strong `Kind`, `SchemaVersion = 1`.
  - 1-based choice numbering: `ChoiceIndexOther = 0` (shared "other"/ "none" sentinel), `ChoiceIndexNone = -1` (skip).
  - Prompt-only `Question`; answers live separately in `AnswerSheet` so an unanswered question has no fake zero answer.
  - `QuestionRound` / `LegendRound` behind the `Round` interface — illegal phase/field mixtures are unrepresentable; frontier kept numeric (`Served` / `Retained`, no prose to parse back).
  - `Validate()` on Question, QuestionRound, LegendRound, AnswerSheet with `errors.Is`-able sentinel errors; per-question errors wrapped with the 1-based position.
  - `Answer.String(choices)` rendering for the agent's chat history.
- **`internal/domain/session`** — the pure grill descriptor: `Active | Closed` status, guarded `Close()` (double legend submission cannot corrupt the archive rules); no daemon/pid state.
- **Docs** — `docs/drafts/contract.md` (single contract, superseding the JSON examples in the old #3), rewritten `docs/issues/3.md`, new `docs/issues/19.md` (state.db payload-version validator), and pointer notes in #1, #4, #5, #13.

## Commits

12 atomic changes, each with its own test-first commit:

1. `feat(round): add markdown/kind vocabulary types`
2. `feat(round): add choice index and recommendation sentinels`
3. `feat(round): add question prompt type`
4. `feat(round): sum-type question and legend rounds`
5. `feat(round): add answer type with rendering`
6. `feat(round): add round invariants and question validate`
7. `feat(round): validate question rounds`
8. `feat(round): add answer sheet and round validation`
9. `feat(session): add pure grill session domain`
10. `docs: add round/answer contract`
11. `docs(issues): rewrite #3 as domain-code contract`
12. `docs(issues): add payload version validator issue`
13. `docs(issues): note contract changes in directly hit issues`

## Deviations from the plan

Documented in `docs/plans/20261023_issue-3-round-domain/deviations.md`; the only substantive one is sub-plan 02 Task 4, where the plan's test data for `TestAnswerSheetValidate` contradicted its own count-mismatch invariant (under-filled probe sheets never reached the `ErrOtherNeedsComment` / `ErrBadAnswerChoice` sentinels). Test probes were corrected to a full answer set per probe; no production code changed.

## Verification

- `go test ./... && go vet ./...` passes at every boundary check (end of sub-plans 01–03 and after the final docs task).
- Table-driven tests cover every exported validator and `String()`; no mocks in domain tests (nothing to mock).

## Dependencies

Design base with no dependencies: #1, #4, #5, #8, #10, #11, #13, #15 consume this contract; #19 extends it with the version validator.
