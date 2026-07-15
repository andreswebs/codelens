---
id: cod-6xou
status: closed
deps: [cod-7s7l, cod-s8uc]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-17 analysis: fragmentation

Analysis: fragmentation (fractal value per entity). Batch D.
New files: src/internal/analysis/fragmentation.go, fragmentation_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; effort.clj as-entity-fragmentation + 7. Depends on P4-14, P4-0.

## Design

Row: type fragmentationRow struct { Entity string; FractalValue float64; TotalRevs int } json entity,fractal_value,total_revs.
Descriptor Name:"fragmentation", Summary:"Author fragmentation (fractal value) per entity". ErrorCodes:["empty_log"].
Algorithm: effort.byEntity; per entity nc=TotalRevs; fractal = 1 - sum over authors of (revs/nc)^2; round via calc.CentiRatio-style 2-sig (original uses ratio->centi-float-precision on (1 - fv1)). Sort [fractal_value, total_revs] DESC.
TDD:

1. TestFragmentation_SingleAuthorZero: one author -> fractal 0.0.
2. TestFragmentation_TwoEqualAuthors: 50/50 -> 1 - (0.25+0.25) = 0.5.
3. TestFragmentation_SortDesc.

## Acceptance Criteria

- fragmentation matches effort_test.clj incl. rounding; desc sort. Cases pass; make validate green.

## Notes

**2026-07-14T13:25:16Z**

Implemented fragmentation analysis (fractal value per entity). New: src/internal/analysis/fragmentation.go + _test.go. Row {entity, fractal_value (float), total_revs}. Algorithm: fractal = 1 - Σ(author_revs/total_revs)² over effort.ByEntity, rounded to 2 sig digits. Sort [fractal_value, total_revs] desc; stable sort preserves entity-asc for full ties (matches original's [fractal-value total-revs] desc). Extracted the 2-sig-digit rounding into new calc.CentiFloat(float64) and refactored calc.CentiRatio to wrap it, since the fractal value is a computed float, not an own/total ratio. Descriptor auto-registers; schema conformance and end-to-end verified. make build green.
