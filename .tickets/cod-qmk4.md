---
id: cod-qmk4
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
# P4-19 analysis: code-age (alias age)

Analysis: code-age (months since last modification; alias age). Batch E.
New files: src/internal/analysis/codeage.go, codeage_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; code_age.clj + code_age_test.clj; design 5.1 (UTC, whole months, --time-now). Depends on P4-0, P2-6.

## Design

Row: type codeAgeRow struct { Entity string; AgeMonths int } json entity,age_months.
Descriptor Name:"code-age", Aliases:["age"], Summary:"Age in months since last modification". Flags:["--time-now"]. ErrorCodes:["empty_log"].
Algorithm: now = parse(--time-now) if set else current date; interpret dates as UTC. Per entity: consider only changes with date STRICTLY BEFORE now; if none, skip entity. latest = max date; AgeMonths = whole calendar months between latest and now (match clj-time in-months). Sort AgeMonths ASC (tiebreak entity asc).
Implement whole-month diff: months = (now.Year-latest.Year)*12 + (now.Month-latest.Month), minus 1 if now.Day < latest.Day (calendar-month semantics matching in-months). Verify against code_age_test.clj expectations.
IF --time-now is not a valid YYYY-MM-dd THEN usage error (exit 2).
TDD - port code_age_test.clj:

1. TestAge_MonthsSinceLatest: known dates + time-now -> expected months.
2. TestAge_IgnoresFutureChanges: change on/after now excluded.
3. TestAge_DefaultNowIsToday (inject now for determinism via --time-now in tests).
4. TestAge_SortAsc.
5. TestAge_BadTimeNow -> usage error exit 2.

## Acceptance Criteria

- code-age matches code_age_test.clj (whole months, UTC, strictly-before-now); --time-now validated; alias age; asc sort. Cases pass; make validate green.

## Notes

**2026-07-14T13:31:09Z**

Implemented code-age (alias age) in src/internal/analysis/codeage.go + codeage_test.go. Row {entity, age_months}. Algorithm: now=--time-now (UTC) or today; per entity take latest change STRICTLY BEFORE now (entities with no qualifying change are dropped); age = whole calendar months via clj-time in-months semantics: months=(now.Y-from.Y)*12+(now.M-from.M), minus 1 when now.Day<from.Day. Sort age_months ASC, entity ASC tiebreak. Bad --time-now => invalid_time_now coded error, exit 2 (added to ErrorCodes alongside empty_log, mirroring messages/invalid_expression precedent). --time-now flag was already wired through Opts.TimeNow in commands.go. No .clj fixtures in repo; tests ported from ticket TDD cases. make build green.
