---
id: cod-rf77
status: closed
deps: [cod-3ksh, cod-jw0u]
links: []
created: 2026-07-14T03:40:09Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-1]
---

# P1-4 gitlog: named parse errors + control-char rejection

Give the parser named, structured errors and control-character rejection.

Edit: src/internal/gitlog/parse.go (+ error vars, e.g. errors.go). Tests in parse_test.go.

Docs: plan.md (Phase 1), design cli-design.md section 5, port reference docs/research/code-maat.md sections 2 (data model), 3 (log format incl 3.4 stacked preludes, 3.5 parser notes). Skills: /golang /tdd.
Reference: requirements section 1 (parse/empty errors) and 14 (control chars), design 5.1, 7. Depends on P1-3 and terr (P0-2).

## Design

Define coded errors (terr):

- var ErrParse = terr.New("parse_error", 3, "generate the log with `codelens print-log-command`", "failed to parse git log")
- var ErrEmptyLog = terr.New("empty_log", 3, "provide a non-empty git2 log on stdin or via --log", "the log is empty")
- var ErrControlChar = terr.New("parse_error", 3, "the input contains disallowed control characters", "invalid control character in input")

Behavior:

- A malformed prelude/numstat line -> wrap ErrParse with fmt.Errorf("%w: entry %d, line %q", ErrParse, idx, line) and attach details via WithDetails({"entry":idx,"line":line}).
- Whole-input empty (0 entries / only whitespace) -> ErrEmptyLog. (Note: a well-formed log with only empty merges yields [] Modifications but is NOT an empty-log error; distinguish "no entries parsed at all" from "entries had no files".)
- Reject NUL and other disallowed control chars (allow \t and \n) anywhere in the input -> ErrControlChar.

TDD cases:

1. TestParse_MalformedNumstat: "..prelude..\nnot-a-numstat\n" -> ErrParse; errors.As Coded exit 3; details has entry index and offending line.
2. TestParse_EmptyLog: ParseString("") -> ErrEmptyLog (exit 3). (Reconcile with P1-3 empty case: choose empty-input -> ErrEmptyLog; update P1-3 test to expect the error, OR keep "" -> [] and treat only stdin-with-no-data as empty. DECIDE: "" -> ErrEmptyLog; whitespace-only -> ErrEmptyLog.)
3. TestParse_ControlChar: input containing "\x00" -> ErrControlChar (exit 3).
4. TestParse_BadDate: prelude with a non-date in the date field -> ErrParse.

## Acceptance Criteria

- Parse failures are coded (parse_error/empty_log, exit 3) with entry/line details.
- NUL/control chars rejected as input error.
- Empty input is empty_log (P1-3 empty test reconciled accordingly).
- All 4 cases pass; make validate green.

## Notes

**2026-07-14T10:59:21Z**

Added coded parser errors + control-char rejection (TDD). New src/internal/gitlog/errors.go defines ErrParse/ErrEmptyLog/ErrControlChar (all exit 3; ErrControlChar shares code parse_error with ErrParse). Parse now: (1) counts tokenized entries and returns ErrEmptyLog only when zero entries (empty/whitespace-only) - an empty-merge prelude with no numstat still counts, so it parses to [] with no error; (2) wraps per-entry failures via parseError() as ErrParse.WithDetails({entry,line}).Wrap(fmt msg), using an unexported entryError{line,err} returned from parseEntry to carry the offending source line (missing prelude->lines[0], bad prelude/date->prelude, bad numstat->that line). Control chars rejected in the tokenizer (tokenize.go hasControlChar) per scanned line, streaming; allows \t and bytes>=0x80, rejects <0x20 and 0x7f. Reconciled the old TestParse_Empty (""->[]) to TestParse_EmptyLog (""->ErrEmptyLog). GOTCHA: errors.Is(err, ErrParse) fails because Wrap/WithDetails return sentinel COPIES; assert coded errors by Code() via errors.As(&terr.Coded) instead. make build green (fmt/vet/lint 0 issues/tests). See learnings.md P1-4.
