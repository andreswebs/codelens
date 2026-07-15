---
id: cod-2boa
status: closed
deps: [cod-8h5b]
links: []
created: 2026-07-14T03:45:34Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-3]
---
# P3-4 transform/teammap: author->team (CSV + JSON)

Team-mapping transform: parse an author->team map (CSV or JSON) and substitute each author with their team; unmapped authors kept as-is.

New files: src/internal/transform/teammap/teammap.go, teammap_test.go.

Docs: plan.md (Phase 3), design cli-design.md 4.2; reference docs/research/code-maat.md sections 4 (pipeline order) and 5 (transforms). Skills: /golang /tdd.
Reference: research 5.3, original app/team_mapper.clj + test/code_maat/app/team_mapper_test.clj. Depends on model (P1-1).

## Design

Surface:

- func Parse(r io.Reader, format string) (map[string]string, error)  // format "csv" (default) or "json"
- func Apply(mods []model.Modification, teams map[string]string) []model.Modification

CSV: header author,team (accept with or without header row; if first row is literally author,team treat as header). Each row maps author->team.
JSON: object {"author":"team",...} OR array [{"author":..,"team":..}]; support the object form primarily, array form for symmetry with group. Malformed -> input error (terr "invalid_team_map", exit 3).
Apply: for each mod, mod.Author = teams[mod.Author] if present else unchanged. Return same length/order.

TDD cases (teammap_test.go):

1. TestParse_CSV: "author,team\nalice,Core\nbob,Core" -> map{alice:Core,bob:Core}.
2. TestParse_JSON_Object: {"alice":"Core"} -> map.
3. TestApply_Remaps: alice->Core substitution; other fields intact.
4. TestApply_UnmappedKept: author not in map stays unchanged.
5. TestParse_Malformed: bad CSV/JSON -> input error (exit 3).
6. TestApply_PortedFixture: reproduce team_mapper_test.clj (incl. unmapped passthrough).

## Acceptance Criteria

- CSV and JSON maps parse; authors remapped; unmapped kept; malformed -> input error (exit 3).
- Ported fixture reproduced. All cases pass; make validate green.

## Notes

**2026-07-14T12:25:54Z**

Implemented transform/teammap (teammap.go + teammap_test.go, testdata/team-map.csv). Parse(r, format) -> map[string]string: format 'csv' (default/'') or 'json'. CSV via encoding/csv, FieldsPerRecord=2 so a wrong-column-count row is a parse error; optional leading 'author,team' header (case-insensitive, trimmed) skipped; values trimmed. JSON dispatches on first non-space byte: '{' -> object map[author]team, '[' -> array of {author,team}, else input error. Malformed CSV/JSON/unknown format -> terr ErrInvalidTeamMap (code invalid_team_map, exit 3) -- note exit 3 (input error) per ticket/design 7.2, unlike group's ErrInvalidGroup which is exit 2. Apply(mods, teams) copies the slice (no input mutation), remaps Author when present, keeps unmapped authors as-is, preserves length+order. Unblocks cod-0xx4 (P3-1 pipeline). make build green.
