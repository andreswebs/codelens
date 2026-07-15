---
id: cod-3ksh
status: closed
deps: [cod-y20g]
links: []
created: 2026-07-14T03:40:09Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-1]
---
# P1-3 gitlog: git2(+subject) entry parser

Parse each git2(+%s) entry into model.Modification records (one per file). This is the faithful port of code-maat's git2 parser.

New files: src/internal/gitlog/parse.go, parse_test.go, plus testdata (see P1-5).

Docs: plan.md (Phase 1), design cli-design.md section 5, port reference docs/research/code-maat.md sections 2 (data model), 3 (log format incl 3.4 stacked preludes, 3.5 parser notes). Skills: /golang /tdd.
Reference: research 3 (format), 3.4 (take LAST of stacked preludes), original test/code_maat/parsers/git2_test.clj (port the exact cases). Depends on P1-2, P1-1.

## Design

Public surface:

- func Parse(r io.Reader, opts model.Options) ([]model.Modification, error)
- func ParseString(s string, opts model.Options) ([]model.Modification, error)  // convenience for tests

Per-entry algorithm:

- Prelude line(s): lines beginning with "--". Format: `--<hash>--<date>--<author>[--<subject...>]`. When multiple prelude lines stack (merge/PR), use the LAST one for rev/date/author/subject. Subject may itself contain "--": split on "--" and rejoin fields[4:] with "--" to reconstruct the subject; missing subject -> Message "-".
- date must match \d{4}-\d{2}-\d{2}; hash matches [0-9a-f]+.
- Remaining lines are numstat: `<added>\t<deleted>\t<path>`. Parse added/deleted: "-" -> LocAdded/LocDeleted 0 with Binary=true; digits -> int; set HasLoc=true. Emit one Modification per numstat line, copying rev/date/author/message.
- An entry with a prelude but no numstat lines yields no Modifications (e.g. empty merges).

Port these exact cases from git2_test.clj (assert on the model equivalents):

- entry: 2 files, rev 990442e, date 2013-08-29, author "Adam Petersen", message "-", locs (1,0) and (2,4).
- binary-entry: project.bin -> Binary true, LocAdded/Deleted 0; second file (2,40).
- entries: two commits -> 6 Modifications total in order (b777738 x2, a527b79 x4).
- empty "" -> [] (nil or empty slice).
- pull-requests: two stacked preludes -> author "Mr Y", rev "77c8751" (the LAST prelude), 2 files.

Additional new cases (design extension):

- Subject present: "--abc--2024-01-02--Jane Doe--Fix parser bug" -> Message "Fix parser bug".
- Subject containing "--": "...--Jane--refactor: split a--b module" -> Message "refactor: split a--b module".

TDD order: single entry -> binary -> multiple -> empty -> pull-requests -> subject -> subject-with-dashes.

## Acceptance Criteria

- All ported git2_test.clj cases reproduced against the model (incl. stacked-prelude -> last prelude).
- Subject captured when present (incl. embedded "--"); defaults to "-" when absent.
- Binary numstat -> Binary=true, 0 locs; text numstat -> ints, HasLoc=true.
- make validate green.

## Notes

**2026-07-14T10:39:43Z**

Implemented gitlog.Parse / ParseString in src/internal/gitlog/parse.go (+parse_test.go). Reduces each blank-line entry to one model.Modification per numstat line. Prelude parsing splits on '--' (leading '--' gives empty fields[0], so hash/date/author = fields[1..3], subject = join(fields[4:], '--') to survive embedded '--'; missing subject -> '-'). Stacked preludes: last one wins (merge/PR parity). hash validated /^[0-9a-f]+$/, date /^\d{4}-\d{2}-\d{2}$/. Numstat: '-' added/deleted -> Binary=true, 0 LOC; digits -> ints; HasLoc always true. Prelude-only entry (empty merge) -> no records. Errors wrapped with 'git log entry N:' prefix as plain fmt errors for now; cod-rf77 will replace these with named terr codes + control-char rejection, and cod-ggqz adds the ported fixture goldens. All 8 ticket TDD cases covered. make build green.
