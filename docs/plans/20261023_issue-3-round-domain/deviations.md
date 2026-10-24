# Deviations from the plan

Recorded during execution of `docs/plans/20261023_issue-3-round-domain`.

## Sub-plan 01 — round vocabulary

- None. All tasks (1–4) were implemented exactly as specified; tests and
  `go vet` pass after each task.
- Process note (not a spec deviation): the Task 3 `jj describe` command was
  interrupted by the user and had to be re-run; no content was affected.
- This file itself was added at the user's request, outside the plan's file
  list.

## Sub-plan 02 — answer and validation

- Task 4, test only: the plan's `TestAnswerSheetValidate` probe sheets for
  `ErrOtherNeedsComment` and `ErrBadAnswerChoice` carried too few answers
  for the submitted round (1 answer vs 2 questions), so the
  `ErrAnswerCountMismatch` rule fired first and the intended sentinel could
  never be reached — the plan's test data contradicts the plan's own
  count-mismatch invariant. Fix: the probe sheets now submit a full set of
  answers (skip on question 1, the violating answer on question 2), so each
  case still exercises exactly its intended sentinel. No production code
  changed.
