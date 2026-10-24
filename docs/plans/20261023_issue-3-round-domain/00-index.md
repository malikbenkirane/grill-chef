# Issue 3 — Round & Answer Domain — Index

Sub-plans (execution order):
- [ ] 01-round-vocabulary — Go module, `internal/domain/round` base types: `Markdown`, `Tag`, `Kind`, `Choice`, `ChoiceIndex`, `Recommendation`, `Question`, round/legend sum types, `SchemaVersion` const
- [ ] 02-answer-and-validation — `Answer`, `AnswerSheet`, `Validate()` on every type, `Answer.String()` rendering
- [ ] 03-session-domain — `internal/domain/session`: `Status` (`Active|Closed`), `Session`, `Close()`
- [ ] 04-contract-and-issue-docs — `docs/drafts/contract.md`, rewrite `docs/issues/3.md`, create `docs/issues/19.md`

Concurrent-eligible batches: `[02-answer-and-validation, 03-session-domain]` (after 01).

Why this order and split: the round vocabulary is the dependency root; validation
and the session domain both build only on 01 and touch disjoint files, so 02 and
03 are the safe unit for one concurrent session each; 04 documents what 01–03
established and must run last. Same-file edits stay within a batch.

This plan executes the shared understanding from the grill session on
`docs/issues/3.md` (legend confirmed by the user). Key amendments of older
documents, deliberately propagated in 04: no round-count cap any more
(frontier closure is the only termination), JSON is an adapter concern only,
and the contract doc supersedes the JSON examples embedded in the old issue 3.
