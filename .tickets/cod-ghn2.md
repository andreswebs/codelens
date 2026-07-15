---
id: cod-ghn2
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:48:27Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-3 analysis: parse (alias identity)

Analysis: parse (raw parsed-record dump; alias identity). Batch A.

New files: src/internal/analysis/parse.go, parse_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (parse); design 6.4 (log order). Depends on P4-0, P2-6.

## Design

Row + descriptor:

- type parseRow struct { Entity,Rev,Date,Author,Message string; LocAdded,LocDeleted *int (omitempty when !HasLoc); Binary bool `json:",omitempty"` } with json tags entity,rev,date,author,message,loc_added,loc_deleted,binary.
- Descriptor{ Name:"parse", Aliases:["identity"], Summary:"Dump parsed modification records (debug/interop)", RowSchema documents all fields, ErrorCodes:["empty_log"], ExitCodes:[0,2,3,1], Run:runParse }
Algorithm: emit records in LOG ORDER (as parsed), no sorting. loc_added/loc_deleted present only when HasLoc; binary true when Binary.

TDD cases:

1. TestParse_LogOrderPreserved: records emitted in input order.
2. TestParse_LocOmittedWhenAbsent: a record with HasLoc=false has no loc fields; with HasLoc=true has ints.
3. TestParse_AliasIdentity: registry Lookup("identity") resolves to parse (assert in registry/schema, but note here).

## Acceptance Criteria

- parse registered with alias identity; log-order dump; loc fields conditional. Cases pass; make validate green.

## Notes

**2026-07-14T13:06:04Z**

Implemented parse analysis (alias identity) in src/internal/analysis/parse.go + parse_test.go. Passthrough dump: emits parsed []Modification verbatim in LOG ORDER, no filtering/aggregation/sorting. parseRow uses *int for loc_added/loc_deleted (omitempty) so records without numstat (HasLoc=false) omit the loc keys entirely rather than reporting misleading zeros; binary is bool omitempty (present only when true). Registered via init with ErrorCodes [empty_log] (empty-log handled upstream by parser), ExitCodes [0,2,3,1]; auto-wired into command tree and schema (verified 'schema --command parse' and 'identity' alias resolves to parse). TDD: log-order preserved, loc omitted-when-absent (asserted via JSON marshaling), binary flag, empty, descriptor+alias. make build green.
