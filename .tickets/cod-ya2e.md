---
id: cod-ya2e
status: closed
deps: []
links: []
created: 2026-07-15T00:46:24Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, architecture, deepening]
---
# Concentrate analysis aggregation loops into calc helpers (MaxBy/Map/FlatMap)

Architecture-review candidate 3, reframed after candidate 1 (which already removed the
envelope boilerplate). The remaining duplication splits by kind:

- **Real logic duplication** — `maindev` and `refmaindev` run an identical
  max-contributor reduce differing only by `.Added` vs `.Deleted`; `maindevbyrevs` runs
  the same reduce over `effort.ByEntity`. The tie-break + total rule is copied three
  times; a fix in one must be mirrored.
- **Structural-only similarity** — the churn trio (`abschurn`/`authorchurn`/
  `entitychurn`) is `SumByGroup` + field-copy + sort; the flatten pair (`ownership`/
  `entityeffort`) is a nested entity x author loop. No shared logic, just shape.

Approach (decided): helper extraction, **no** table-driven descriptor constructor. A
constructor would force the shared row to be a `map[string]any` (JSON keys sort
alphabetically -> field-order change) or per-analysis typed structs (no gain), and
would hide each analysis's row schema behind a spec. So we keep every Descriptor, row
struct, and sort **explicit** and concentrate only the loop bodies into small generic
helpers in `calc`.

Skills: /tdd (helpers test-first, then refactor under existing golden tests),
/golang (generics, table-driven tests), /llm-coding (surgical, no behavior change,
verifiable success).

## Design

### Parity constraint (why no constructor)

`internal/output/format.go`: the `json` format marshals typed row structs (field order
= struct field order); `csv`/`table` read values by JSON key (`rowMaps`) with column
order from `RowSchema`. A `[]map[string]any` row would reorder JSON fields
alphabetically (e.g. `date,added,deleted,commits` -> `added,commits,date,deleted`),
changing output bytes. Typed row structs stay; helpers operate over them.

### New helpers in `internal/analysis/calc/calc.go`

Sits with the existing generic aggregation helpers (`GroupBy`, `Distinct`).

```go
// MaxBy returns the first element of items with the greatest val(item) and the
// sum of val over all items. "First" means ties resolve to the earliest element,
// so a caller that pre-sorts items (e.g. ascending author) gets a deterministic
// winner. items must be non-empty; every analysis calls it per entity, which
// always has at least one contributor.
func MaxBy[T any](items []T, val func(T) int) (top T, total int)

// Map returns f applied to each element of src, preserving order.
func Map[S, R any](src []S, f func(S) R) []R

// FlatMap returns the concatenation of f applied to each element of src,
// preserving order.
func FlatMap[S, R any](src []S, f func(S) []R) []R
```

`MaxBy` semantics must reproduce the current loops exactly: `top := items[0]`,
`total := 0`, then for each item `total += val(item)` and `if val(item) > val(top) {
top = item }` (strict `>`, keeps first on tie).

### Apply per cluster (contracts unchanged)

**Reduce family (core value):**

- `maindev.go`: `top, total := calc.MaxBy(e.Contribs, func(c churn.AuthorContrib) int
  { return c.Added })`; row uses `top.Author`, `top.Added`, `total`,
  `calc.CentiRatio(top.Added, total)`. Wrap the per-entity build in
  `calc.Map(churn.ByEntityAuthorContrib(mods), ...)`.
- `refmaindev.go`: identical but `return c.Deleted` and the removed-line fields.
- `maindevbyrevs.go`: `top, _ := calc.MaxBy(e.Authors, func(a effort.AuthorRevs) int {
  return a.Revs })`; keep using `top.TotalRevs` for the total (already computed per
  author; do **not** substitute MaxBy's sum, to preserve exact parity). No loc guard.

**Churn trio (stylistic):** replace the map loop with
`rows := calc.Map(churn.SumByGroup(mods, key), func(g churn.GroupChurn) xRow { ... })`.
`RequireLoc` guard and the per-analysis sort stay.

**Flatten pair (stylistic):**
`rows := calc.FlatMap(entities, func(e ...) []xRow { return calc.Map(e.Authors, ...) })`
(ownership over `churn.ByEntityAuthorContrib`, entityeffort over `effort.ByEntity`).
Sorts stay.

> Honest note: the churn-trio and flatten-pair changes concentrate no logic — they
> trade an explicit `for` loop for a closure. Included because both clusters were in
> scope; if a loop reads clearer than `Map`/`FlatMap` in review, keep the loop for that
> file. `MaxBy` is the load-bearing extraction and should land regardless.

### TDD plan (/tdd)

1. Red/green the helpers first in `calc/calc_test.go`:
   - `MaxBy`: picks the max; ties resolve to the first (pre-sorted input); `total` is
     the sum; single-element slice; negative/zero values.
   - `Map`/`FlatMap`: order preserved; empty input -> empty (non-nil where the loop
     produced `make([]R, 0, ...)`); nested `FlatMap(Map(...))` shape.
2. Refactor each analysis Run to use the helpers. The existing per-analysis tests and
   CLI golden tests are the parity guard — they must stay green with **no expectation
   changes** (pure internal refactor, byte-identical output).
3. `make build` green.

### Out of scope

- No descriptor constructor; no change to any `Descriptor`, row struct, `RowSchema`, or
  sort order.
- No merge of `effort` into `churn` (that is candidate 5).
- No output/behavior change of any kind.

### Files touched

```text
internal/analysis/calc/calc.go            (add MaxBy, Map, FlatMap)
internal/analysis/calc/calc_test.go       (helper tests)
internal/analysis/maindev.go              (MaxBy + Map)
internal/analysis/refactoringmaindev.go   (MaxBy + Map)
internal/analysis/maindevbyrevs.go        (MaxBy + Map; keep top.TotalRevs)
internal/analysis/abschurn.go             (Map)
internal/analysis/authorchurn.go          (Map)
internal/analysis/entitychurn.go          (Map)
internal/analysis/ownership.go            (FlatMap + Map)
internal/analysis/entityeffort.go         (FlatMap + Map)
```

## Acceptance Criteria

- `calc.MaxBy`, `calc.Map`, `calc.FlatMap` exist with tests covering tie-break (first
  wins on pre-sorted input), total sum, order preservation, and empty input.
- `maindev`, `refmaindev`, and `maindevbyrevs` obtain their main contributor via a
  single `calc.MaxBy` call; the tie-break + total rule is defined in exactly one place.
  `maindevbyrevs` still reports `top.TotalRevs` as the total.
- The churn trio and flatten pair build rows via `Map`/`FlatMap` (or keep an explicit
  loop where a reviewer finds it clearer); their Descriptors, row structs, and sorts
  are unchanged.
- No `Descriptor`, `RowSchema`, or sort order changes; every existing analysis and CLI
  test passes with unchanged expectations (byte-identical output).
- `make build` green (validate + compile).

## Notes

**2026-07-15T00:51:34Z**

Added generic helpers calc.MaxBy/Map/FlatMap (calc.go) with table-driven tests in calc_test.go (max/sum, tie-keeps-first on pre-sorted input, single-element, negative/zero, order preservation, non-nil empty slices, nested FlatMap(Map)). Refactored the reduce family (maindev, refmaindev, maindevbyrevs) onto a single MaxBy call so the tie-break+total rule lives in one place; maindevbyrevs still reports top.TotalRevs (not MaxBy's sum) to avoid double-counting the entity-wide total repeated per author row. Churn trio (abschurn/authorchurn/entitychurn) and flatten pair (ownership/entityeffort) now build rows via Map/FlatMap; Descriptors, row structs, RequireLoc guards, and sorts unchanged. Pure internal refactor: all existing analysis + CLI golden tests pass byte-identical, no expectation changes. make build green (vet, golangci-lint 0 issues, tests, compile).
