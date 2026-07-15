---
id: cod-en35
status: closed
deps: [cod-8h5b]
links: []
created: 2026-07-14T03:45:33Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-3]
---
# P3-3 transform/temporal: sliding N-day grouping

Temporal-period transform: collapse commits within a sliding N-day window into single logical change sets (rev = window latest date, dedupe by entity).

New files: src/internal/transform/temporal/temporal.go, temporal_test.go.

Docs: plan.md (Phase 3), design cli-design.md 4.2; reference docs/research/code-maat.md sections 4 (pipeline order) and 5 (transforms). Skills: /golang /tdd.
Reference: research 5.2, original app/time_based_grouper.clj + test/code_maat/app/time_based_grouper_test.clj. Depends on model (P1-1).

## Design

Surface:

- func Apply(mods []model.Modification, periodDays int) ([]model.Modification, error)

Algorithm (port of time-based-grouper):

- If periodDays < 1 -> usage error (terr "invalid_temporal_period", exit 2).
- If mods empty -> return empty.
- Determine first/last commit date (parse YYYY-MM-dd as UTC). Build the full daily date range [first,last].
- Group commits by date; pad missing days with empty sets so every day in range is present.
- Slide a window of periodDays (step 1) across the ordered days (partition size=periodDays, step 1). For each window: merge all commits in the window into one change set; set every record's Rev to the window's latest date (string); dedupe by Entity (keep first occurrence). Skip windows that are entirely empty.
- Return the flat sequence of all windows' records.

NOTE: this intentionally re-counts a physical commit across overlapping windows (correct for coupling, not for counts). Document that.

TDD cases (temporal_test.go):

1. TestApply_InvalidPeriod: periodDays 0 or negative -> usage error.
2. TestApply_Empty: [] -> [].
3. TestApply_Period1_IsPerDayGrouping: period 1, two commits same day -> merged into one change set (rev=that date), duplicate entities deduped.
4. TestApply_Period2_SlidingOverlap: three consecutive days with distinct files -> overlapping windows merge adjacent days; assert record membership and rev=window latest date.
5. TestApply_DedupeByEntity: same entity changed on two days within a window -> appears once, rev=latest date.
6. TestApply_PortedFixture: reproduce a case from time_based_grouper_test.clj.

## Acceptance Criteria

- Sliding N-day windows built with padding; merged; rev set to latest date; deduped by entity; empty windows skipped.
- period<1 -> usage error. Ported fixture case reproduced. All cases pass; make validate green.

## Notes

**2026-07-14T12:22:26Z**

Implemented transform/temporal (Apply(mods, periodDays) ([]Modification, error)). Faithful port of code-maat time_based_grouper: pad daily range [first,last]; slide window size=periodDays step=1 (Clojure partition n 1 semantics -> incomplete trailing window dropped, so a range shorter than the period yields empty output); each window merges commits into one change set with Rev=window latest calendar day (even if that day had no commits, thanks to padding), deduped by Entity keeping the earliest occurrence (other fields from that occurrence, only Rev overwritten). Empty windows skipped naturally (mergeWindow returns nil). Dates parsed as UTC via time.ParseInLocation. Errors: period<1 -> invalid_temporal_period (exit 2, usage); non-YYYY-MM-dd Date -> invalid_temporal_date (exit 3, input). NOTE: upstream corpus symlink (.local/refs/code-maat) is broken in this env, so the ported-fixture test is a representative padding+skip scenario grounded in research 5.2 + the ticket algorithm rather than a byte copy of time_based_grouper_test.clj. make build green. Next: cod-2boa (teammap) still needed to unblock cod-0xx4 pipeline.
