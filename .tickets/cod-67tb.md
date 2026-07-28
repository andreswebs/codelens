---
id: cod-67tb
status: open
deps: [cod-eex1]
links: []
created: 2026-07-28T16:50:08Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, spec-004, go, schema]
---
# Publish aggregation_roles in codelens schema, in both the list and per-command forms

Publish `aggregation_roles` in `codelens schema` output, in BOTH forms.

Spec: docs/specs/004-aggregation-roles/plan.md section 6.2 and 7.3 (ticket C of five).

## Why exposure is required rather than nice-to-have

NO GO CODE AGGREGATES OUTPUT ROWS. Verified: every transform (`group`, `temporal`,
`teammap`) operates on `[]model.Modification` upstream of analysis, so each analysis
re-derives its measures from transformed events and there is no row-merging step. A
Go-only `AggRoleOf` therefore has zero consumers that aggregate anything.

Every aggregation in the system is in Python, and Python cannot call a Go function. This
ticket is the only channel by which roles reach the code that could violate them.

## Why it also fixes an existing hole

Today `codelens schema` publishes `schema_version`, `ok`, `commands`, `errors` and
enumerates the 12 semantics NOWHERE: they appear only per-column inside
`row_schema[].semantic`. A semantic-to-role map in the list form implicitly publishes the
closed set for the first time.

## Design

## Shape: both forms, mirroring error codes

The precedent is exact and is the reason to prefer this over the alternatives. Error codes
are published as a full global catalog in the list form (`errors`) AND as a per-command
subset (`error_codes`) plus the shared remainder (`common_error_codes`). Aggregation roles
are the same kind of thing: a closed global vocabulary with per-command relevance.

```jsonc
// codelens schema                       -- the full 12-entry catalog
{ "schema_version": 1, "ok": true, "commands": [...], "errors": [...],
  "aggregation_roles": {
    "count": "additive",     "loc": "additive",
    "percentage": "intensive", "ratio": "intensive",
    "duration_months": "intensive",
    "filepath": "dimension", "person": "dimension", "date": "dimension",
    "label": "dimension",    "flag": "dimension",
    "commit_id": "identifier", "text": "identifier" } }

// codelens schema --command revisions   -- only what this command's columns use
{ "command": "revisions",
  "row_schema": [ {"name": "entity", "semantic": "filepath"},
                  {"name": "n_revs", "semantic": "count"} ],
  "aggregation_roles": { "filepath": "dimension", "count": "additive" } }
```

## Three properties that were reasons to choose this shape

1. ONE `--schema` FILE STILL SUFFICES for the Python guard, because the per-command form
   carries the roles its own columns need. Had the catalog gone only in the list form, the
   guard would need a second input and would reopen the settled second-input decision
   (shared with spec 003's Q5).
2. It is a MAP, NOT A PER-COLUMN FIELD. Repeating a per-semantic fact on each column would
   restate it up to seven times for one command and re-create the drift risk in the wire
   format. A consumer joins on `row_schema[].semantic`, a one-line lookup.
3. The list-form map is itself the wire-level exhaustiveness check: its golden pins all
   twelve semantics, so a thirteenth cannot be added without updating it.

## Two details settled by existing behaviour, not by new decision

- FLAG-GATED COLUMNS ARE INCLUDED in the per-command subset. `Column.FlagGated` is
  `json:"-"` precisely because the schema declares the full untransformed vocabulary,
  gated columns included. So roles must cover every DECLARED column, not only the ones a
  hypothetical invocation would emit. `schema` describes the command; `semantics` in the
  envelope describes the run.
- THE KEY NAME IS `aggregation_roles` IN BOTH FORMS. Error codes use different names
  (`errors` versus `error_codes`) because their CONTENT differs: full objects with hints
  versus bare codes. Roles have identical content in both forms, differing only in scope,
  so one name is correct.

## Envelope is NOT touched

`semantics` stays a flat `field -> string` map in the result envelope. The Flint adapter's
near-identity property must remain intact. No `schema_version` bump: adding a key breaks
no documented field.

## Golden files

`internal/command/testdata/schema_list.out` and every `*_schema.out` (currently
`authors_schema.out`) change. Additive only. Review the diffs rather than blind-updating.

## Acceptance Criteria

- Full 12-entry `aggregation_roles` map in `codelens schema`.
- Per-command `aggregation_roles` subset in `codelens schema --command CMD`, covering
  every DECLARED column including flag-gated ones.
- Same key name in both forms.
- The result envelope is unchanged: `semantics` is still a flat `field -> string` map, no
  `aggregation_roles` key, no `schema_version` bump.
- Golden updates to `schema_list.out` and every `*_schema.out`, additive only.
- A test asserting the list-form map covers every member of `Semantics()`, so the golden
  is a genuine wire-level exhaustiveness check.
- `make build` green.

