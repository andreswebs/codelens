---
id: cod-8h5b
status: closed
deps: []
links: []
created: 2026-07-14T03:40:09Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-1]
---
# P1-1 model: Modification and Options types

Define the core data types the parser produces and every analysis consumes.

New files: src/internal/model/model.go, model_test.go.

Docs: plan.md (Phase 1), design cli-design.md section 5, port reference docs/research/code-maat.md sections 2 (data model), 3 (log format incl 3.4 stacked preludes, 3.5 parser notes). Skills: /golang /tdd.

## Design

model.go:

- type Modification struct {
    Entity   string  // changed file path
    Rev      string  // commit short hash
    Date     string  // YYYY-MM-dd (canonical)
    Author   string  // committer name (may be remapped to a team downstream)
    Message  string  // commit subject; "-" when absent (stock 3-field log)
    LocAdded   int   // lines added; 0 for binary
    LocDeleted int   // lines deleted; 0 for binary
    Binary   bool    // git reported "-"/"-" (binary file)
    HasLoc   bool    // numstat present (guards churn analyses; always true for git2)
  }
- type Options struct { InputEncoding string; /*other run options added by later tickets*/ }

Notes: keep the struct free of JSON tags here; output shaping (snake_case) is the analysis/output layer's concern. This is a pure data type with no behavior beyond maybe a String() for debugging (optional).

TDD cases (model_test.go):

1. TestModification_ZeroValue: zero value has empty strings, 0 locs, Binary=false, HasLoc=false (documents the shape). This is a light guard; the real coverage comes from the parser tests.

## Acceptance Criteria

- model package compiles; Modification and Options exported and documented.
- make validate green.

## Notes

**2026-07-14T10:32:46Z**

Added src/internal/model/{model.go,model_test.go}. Modification struct: Entity, Rev, Date (canonical YYYY-MM-dd string), Author, Message ('-' when absent), LocAdded/LocDeleted (int; 0 for binary), Binary and HasLoc flags. Options struct holds InputEncoding only (empty == UTF-8 default); later tickets extend it. Pure data types, no JSON tags (output shaping is the analysis/output layer's job). TDD zero-value guard tests only, per ticket; real field coverage lands with the parser (cod-y20g/cod-3fh4). make build green.
