---
id: cod-ic8y
status: closed
deps: [cod-f0zl, cod-g7yh]
links: []
created: 2026-07-14T03:52:50Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-5]
---
# P5-4 cmd: usage-error classification -> exit 2

Ensure usage errors from the CLI framework map to exit 2 with a coded JSON error envelope on stderr.
Edit: src/internal/output/errors.go classifyUsageError for urfave/cli v3 messages; wire in run().
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: cli-design.md 7, requirements 10, docs/research/urfave-cli.reference.md (CLI framework error strings). Depends on P2-3, P0-3.

## Design

- classifyUsageError recognizes urfave/cli v3 error strings: unknown flag, missing required flag, invalid flag value, unknown command/subcommand. Map to codes (unknown_flag, missing_required_flag, invalid_value, unknown_command) all exit 2 with a helpful hint (check --help).
- Verify `messages` without --expression -> usage error exit 2 (missing_required_flag) BEFORE any parsing side effects.
TDD:

1. TestUsage_UnknownFlag -> exit 2, code unknown_flag.
2. TestUsage_UnknownSubcommand -> exit 2.
3. TestUsage_InvalidIntFlag (e.g. --rows abc) -> exit 2 invalid_value.
4. TestUsage_MessagesMissingExpression -> exit 2 missing_required_flag.

## Acceptance Criteria

- All usage-error classes map to exit 2 with coded JSON envelope + hint on stderr. Cases pass; make validate green.

## Notes

**2026-07-14T13:46:39Z**

Usage-error classification refined to distinct coded errors, all exit 2 with a coded JSON envelope + hint on stderr and clean stdout. classifyUsageError (internal/output/errors.go) now maps urfave/cli v3 message substrings: 'flag provided but not defined'/'no such flag' -> unknown_flag; 'invalid value' -> invalid_value; 'Required flag'/'not set' -> missing_required_flag. Unknown commands are coded unknown_command in run() (was usage_error). Key fix: urfave by default dumps 'Incorrect Usage' + full help (help went to stdout!) on parse errors; setting each command's OnUsageError hook to a passthrough (return err) suppresses both. The hook is NOT inherited, so it's set on root + every subcommand (analysis cmds, print-log-command, schema). messages without --expression is enforced by urfave before Run (no parse side effects) and now reports missing_required_flag. Tests: internal/output/errors_test.go (table over 4 classes) + new cmd/codelens/usage_error_test.go (TestUsage_UnknownFlag/UnknownSubcommand/InvalidIntFlag/MessagesMissingExpression, each asserting exit 2 + empty stdout + code). The other usage_error codes (invalid --after, unknown schema cmd, invalid --format) are deliberate app-level errors, left unchanged. make build green.
