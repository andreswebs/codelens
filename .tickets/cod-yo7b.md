---
id: cod-yo7b
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-20 analysis: messages

Analysis: messages (entity frequency for commit-message matches). Batch E.
New files: src/internal/analysis/messages.go, messages_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; commit_messages.clj + commit_messages_test.clj; requirements 2, 14 (bounded regex). Depends on P4-0, P2-6.

## Design

Row: type messagesRow struct { Entity string; Matches int } json entity,matches.
Descriptor Name:"messages", Summary:"Entity frequency for commit-message regex matches". Flags:["--expression"(required)]. ErrorCodes:["empty_log","missing_messages","invalid_expression"], ExitCodes:[0,2,3,1].
Algorithm: require --expression; compile with a length/complexity bound -> invalid_expression usage error (exit 2) if invalid/oversized (Go RE2 is linear so bound is the main guard). ensure-supported-vcs: if there are rows but EVERY Message == "-" -> ErrMissingMessages (input error exit 3, hint about generating the log with --%s / print-log-command). Keep rows whose Message matches the regex; count per entity; sort [matches, entity] DESC.
TDD - port commit_messages_test.clj:

1. TestMessages_CountsMatchesPerEntity: expression "bug" -> entities with matching commit messages counted.
2. TestMessages_MissingExpression -> usage error exit 2.
3. TestMessages_NoMessagesLog: all messages "-" -> missing_messages exit 3.
4. TestMessages_InvalidExpression -> invalid_expression exit 2.
5. TestMessages_SortMatchesDesc.

## Acceptance Criteria

- messages matches commit_messages_test.clj; requires+bounds --expression; errors on message-less log (exit 3). Cases pass; make validate green.

## Notes

**2026-07-14T12:53:32Z**

Implemented messages analysis (src/internal/analysis/messages.go + _test.go). Row {entity,matches}; counts distinct matching revisions per entity; sort [matches,entity] DESC (entity desc, faithful to code-maat -order-by :desc; entity is the group key so fully deterministic). --expression required (string); compiled with a maxExpressionLen=1000 bound. Errors: ErrInvalidExpression (invalid_expression, exit 2 - empty/oversized/uncompilable) and ErrMissingMessages (missing_messages, exit 3 - log where every Message=="-"). Empty mod slice is not an error (empty_log handled upstream by parser). Flag wiring (--expression -> Opts.Expression) already existed in cmd/commands.go. Verified e2e via CLI: match, missing-flag exit 2, missing_messages exit 3, invalid_expression exit 2, and schema --command messages. make build green. Unblocks cod-qoff.
