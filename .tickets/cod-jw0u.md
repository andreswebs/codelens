---
id: cod-jw0u
status: closed
deps: []
links: []
created: 2026-07-14T03:37:08Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-0]
---

# P0-2 internal/terr: coded errors

Port the coded-error model that every command uses for structured errors and exit-code mapping. Mirrors the sibling terminology repo's internal/terr (reimplemented; codelens is GPL-3.0).

New files: src/internal/terr/terr.go, src/internal/terr/terr_test.go.

Docs (repo-root relative):

- Plan task: docs/specs/001-initial-implementation/plan.md (Phase 0)
- Design: docs/cli-design.md
- Requirements: docs/specs/001-initial-implementation/requirements.md
- Go style: /golang skill. TDD: /tdd skill (vertical slices, one test -> one impl).
  Design ref: cli-design.md section 7 (Errors and exit codes).

## Design

Public surface:

- type Coded interface { error; Code() string; ExitCode() int; Hint() string }
- type Detailed interface { ErrorDetails() any } // optional, for structured details
- type Error struct { code string; exit int; hint, msg string; wrapped error }
- func New(code string, exit int, hint, msg string) \*Error
- (\*Error) Error() string // msg; if wrapped != nil -> msg + ": " + wrapped.Error()
- (\*Error) Code()/ExitCode()/Hint() accessors
- (\*Error) Unwrap() error // returns wrapped
- func (e *Error) WithDetails(any)*Error // returns a copy implementing Detailed

Usage pattern (like terminology): define package-level sentinels, e.g.
var ErrParse = terr.New("parse_error", 3, "generate the log with `codelens print-log-command`", "failed to parse git log")
then wrap with context: fmt.Errorf("%w: entry %d", ErrParse, n). errors.As must recover the \*Error to read Code/ExitCode/Hint.

TDD cases (terr_test.go), one at a time:

1. TestNew_Accessors: New("c",3,"h","m") -> Code()=="c", ExitCode()==3, Hint()=="h", Error()=="m".
2. TestError_WrappedMessage: New(...).with wrapped -> Error() == `"m: <wrapped>"`.
3. TestErrorsAs_RecoversCoded: e := fmt.Errorf("%w: ctx", base); var c terr.Coded; errors.As(e,&c) true; c.Code()==base.Code(); c.ExitCode()==base.ExitCode().
4. TestUnwrap: errors.Unwrap(New with wrapped) == wrapped; errors.Is(wrappedChain, base) true.
5. TestWithDetails_ImplementsDetailed: base.WithDetails(map[string]any{"entry":4}) -> var d terr.Detailed; errors.As(...,&d) true; d.ErrorDetails() deep-equals the map. Original base unchanged (copy semantics).

## Acceptance Criteria

- terr package compiles; Coded/Detailed satisfied by \*Error.
- All 5 test cases pass; errors.As/Is/Unwrap behave as specified.
- Exported symbols have doc comments; make validate green.

## Notes

**2026-07-14T10:16:55Z**

Implemented internal/terr coded-error package (TDD, 5 cases green). Public surface: Coded{error,Code,ExitCode,Hint} and Detailed{ErrorDetails} interfaces; *Error with New(code,exit,hint,msg). Added Wrap(err) and WithDetails(any) as copy-returning methods (receiver unchanged so package-level sentinels stay reusable) - the design's 'with wrapped' path is Wrap. Error() appends `': <wrapped>'` when set; Unwrap() exposes the cause so errors.Is/As traverse. Usage pattern: define sentinels, wrap with fmt.Errorf("%w: ctx", sentinel); errors.As recovers the*Error pointer. Compile-time asserts `_Coded`/`_Detailed` = `(*Error)(nil)`. Unblocks cod-g7yh (P0-3 output envelope), cod-kyfe, cod-rf77. make build green.
