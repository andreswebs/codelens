---
id: cod-xx0y
status: closed
deps: [cod-f0zl]
links: []
created: 2026-07-14T03:52:50Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-5]
---
# P5-1 cmd: print-log-command helper

Implement the print-log-command helper: emit the exact git log command to generate a compatible (extended git2 + %s) log.
New files: src/cmd/codelens/printlogcommand.go, printlogcommand_test.go.
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: cli-design.md 5, requirements 9. Depends on P2-3 (command tree).

## Design

Command `print-log-command` (no positional args). Optional flag --after DATE (YYYY-MM-dd).
Output: the single git command, e.g.:
  git log --all --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames
When --after set, append " --after=DATE". Print to stdout (plain text, not JSON - it is a copy-paste command). Exit 0.
IF --after is not a valid YYYY-MM-dd THEN usage error exit 2.
TDD:

1. TestPrintLogCommand_Default: stdout equals the exact expected command string incl the --%s subject.
2. TestPrintLogCommand_After: --after 2024-01-01 appends --after=2024-01-01.
3. TestPrintLogCommand_BadAfter -> exit 2.

## Acceptance Criteria

- Emits the exact extended-git2 log command; --after appends the window; bad date -> exit 2. Cases pass; make validate green.

## Notes

**2026-07-14T11:12:28Z**

Implemented print-log-command in src/cmd/codelens/printlogcommand.go (+printlogcommand_test.go). Standalone top-level command (NOT a registry analysis) wired via main.go: Commands: append(analysisCommands(stdin), printLogCommand()). Emits the exact extended-git2+%s command from logCommandBase const to cmd.Root().Writer (plain text, not JSON, since it is copy-paste). --after DATE validated with a strict round-trip time.Parse("2006-01-02") check (rejects unpadded like 2024-1-1 and out-of-range like 2024-13-99); bad value -> errBadAfter coded usage_error (exit 2) on stderr, nothing on stdout. Global root flags (--log/--format/etc.) are inherited by this command per existing architecture but ignored. 3 TDD cases green; verified end-to-end that the emitted command round-trips into 'authors'. make build green. Unblocks cod-tyux (AGENTS.md), cod-vdoq (README), cod-3fh4 (DoD).
