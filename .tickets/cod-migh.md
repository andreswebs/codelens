---
id: cod-migh
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:49:32Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---

# P4-7 analysis: churn core helpers

Shared churn helpers used by all churn analyses: loc-required guard, sum-by-group, per-author contribution aggregation.

New files: src/internal/analysis/churn/churn.go, churn_test.go.

Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd.
Reference: original analysis/churn.clj + churn_test.clj. Depends on P4-0, P2-6.

## Design

Helpers (port churn.clj):

- var ErrMissingMetrics = terr.New("missing_metrics", 3, "generate the log with --numstat (see `codelens print-log-command`)", "the VCS data has no modification metrics")
- func requireLoc(mods) error : error if the data has no loc at all (all HasLoc=false). (git2 always has loc; guard defends against message-only logs.)
- type groupChurn struct { Group string; Added, Deleted, Commits int }
- func sumByGroup(mods, key func(model.Modification) string) []groupChurn : per group sum Added, Deleted (binary counts 0), Commits = distinct Rev count. Deterministic key order.
- type authorContrib struct { Author string; Added, Deleted int }
- func byEntityAuthorContrib(mods) []struct{ Entity string; Contribs []authorContrib } : group by entity, then per author sum added/deleted.

TDD cases (churn_test.go):

1. TestRequireLoc_ErrorsWhenAbsent: mods with HasLoc=false -> ErrMissingMetrics (exit 3).
2. TestSumByGroup_SumsAndDistinctCommits: two revs touching a group -> Added/Deleted summed, Commits=2.
3. TestSumByGroup_BinaryCountsZero: binary rows contribute 0 to added/deleted.
4. TestByEntityAuthorContrib: per-entity per-author sums correct.

## Acceptance Criteria

- churn helpers match churn.clj; loc guard errors (exit 3); binary=0; distinct-commit counts. Cases pass; make validate green.

## Notes

**2026-07-14T11:49:11Z**

Implemented churn core helpers in new subpackage internal/analysis/churn (churn.go + churn_test.go, package churn).

Contract delivered:

- ErrMissingMetrics: terr coded error (code=missing_metrics, exit=3, hint points to print-log-command).
- requireLoc(mods): errors ONLY when mods is non-empty AND every mod has HasLoc=false. Empty slice returns nil (empty_log is handled upstream, not a metrics concern). git2 always has loc, so this only bites message-only logs.
- groupChurn{Group,Added,Deleted,Commits} + sumByGroup(mods,key): sums LocAdded/LocDeleted per group, Commits=distinct Rev count. Binary rows already normalized to 0 loc by the parser so they naturally contribute 0 while still counting as a commit. Uses calc.GroupBy so groups come back ascending-key-ordered (deterministic).
- authorContrib{Author,Added,Deleted}, entityContribs{Entity,Contribs}, byEntityAuthorContrib(mods): entity->author nested roll-up, both levels ascending-key-ordered (for entity-ownership).

Notes for downstream churn analyses (cod-asdr/c52u/sg05/rq5s/x5r6/7w03):

- Helpers are UNEXPORTED, so those analyses must live in THIS package (internal/analysis/churn) to reuse them, OR the helpers get promoted/exported when wired. Registry-registration + per-analysis sort orders (see reference doc section 6: abs-churn sort [date,added,deleted] asc; author-churn [author,added] asc; entity-churn added desc; entity-ownership entity asc) are NOT here yet - this ticket is helpers only.
- Chose a named type entityContribs instead of the anonymous struct in the ticket signature (idiomatic + avoids anon-struct in signatures). Unexported types are fine for revive.
- unused(U1000) does not fire because churn_test.go (same package) exercises every helper.

make build green.
