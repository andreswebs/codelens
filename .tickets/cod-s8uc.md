---
id: cod-s8uc
status: closed
deps: [cod-8h5b]
links: []
created: 2026-07-14T03:48:27Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-0 analysis/calc: grouping + rounding helpers

Shared aggregation and rounding helpers used by all Phase 4 analyses. Rounding is load-bearing for numeric parity, so pin it first (before any analysis depends on it).

New files: src/internal/analysis/calc/calc.go, calc_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 7 (average, percentage, int-truncation, ratio->centi-float-precision). Depends on model (P1-1).

## Design

Exported helpers (calc package):

- func GroupBy[T any](xs []T, key func(T) string) *ordered groups  // deterministic: preserve first-seen key order OR sort keys; document choice (sort keys ascending for determinism).
- func Distinct[T comparable](xs []T) []T
- func Average(a, b int) float64            // (a+b)/2 as float64
- func Percentage(v float64) float64        // v*100
- func TruncInt(v float64) int              // truncation toward zero (Go int() semantics == original (int ...))
- func Ceil(v float64) int                  // math.Ceil -> int
- func CentiRatio(own, total int) float64   // 2 SIGNIFICANT digits of own/max(total,1); reproduces ratio->centi-float-precision

CentiRatio impl: r := float64(own)/float64(max(total,1)); round to 2 significant digits. Use strconv.FormatFloat(r,'g',2,64) then ParseFloat, or an equivalent significant-digit rounding. Verify exactly against the examples below.

TDD cases (calc_test.go) - pin rounding to research 7 examples:

1. TestAverage: Average(44,45)=44.5; Average(10,10)=10.
2. TestPercentage_TruncInt_Degree: shared=35 avg=44.5 -> Percentage(35/44.5)=78.65..; TruncInt=78 (matches coupling degree example).
3. TestCeil_AverageRevs: Ceil(44.5)=45; Ceil(44.0)=44.
4. TestCentiRatio_TwoSigDigits: CentiRatio(834,1000)=0.83; CentiRatio(834,10000)=0.083; CentiRatio(1,3)=0.33; CentiRatio(5,0)=CentiRatio(5,1)=5.0.
5. TestGroupBy_Deterministic: grouping yields keys in a stable (sorted) order across runs.
6. TestDistinct_PreservesFirstSeen (or sorted - document).

## Acceptance Criteria

- Rounding helpers reproduce every research-7 example exactly (2 significant digits, ceil, int truncation).
- GroupBy/Distinct deterministic. All cases pass; make validate green.

## Notes

**2026-07-14T10:46:19Z**

Implemented src/internal/analysis/calc/{calc.go,calc_test.go} (new package). Exported helpers: GroupBy[T] (returns []Group[T] sorted ascending by key for determinism; items keep first-seen order), Distinct[T comparable] (preserves first-seen), Average(a,b int) float64, Percentage(v) v*100, TruncInt (math.Trunc, toward-zero == original int()), Ceil (math.Ceil->int), CentiRatio(own,total) (2 significant digits via FormatFloat 'g' prec 2 -> ParseFloat; total<1 clamped to 1). All research-7 rounding examples pinned exactly incl CentiRatio(834,1000)=0.83, (834,10000)=0.083, (1,3)=0.33, (5,0)=(5,1)=5.0; coupling degree TruncInt(Percentage(35/44.5))=78; Ceil(44.5)=45. Design note: GroupBy sorts KEYS ascending (documented divergence from original insertion order) so downstream sort/--rows truncation is deterministic. TDD red->green; make build green. Unblocks the 13 Phase-4 analysis tickets.
