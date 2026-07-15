---
id: cod-kyfe
status: closed
deps: [cod-8h5b, cod-jw0u]
links: []
created: 2026-07-14T03:45:33Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-3]
---
# P3-2 transform/group: layer mapping (text + JSON)

Architectural grouping transform: parse a layer-mapping spec (text '=>' or JSON) and remap each entity to the first matching logical group, dropping unmatched entities.

New files: src/internal/transform/group/group.go, group_test.go, testdata (ported layer defs).

Docs: plan.md (Phase 3), design cli-design.md 4.2; reference docs/research/code-maat.md sections 4 (pipeline order) and 5 (transforms). Skills: /golang /tdd.
Reference: research 5.1 (anchoring + drop-unmatched), original app/grouper.clj + test/code_maat/app/grouper_test.clj + end_to_end/*layers-definition.txt. Depends on model (P1-1) and terr (P0-2).

## Design

Types + surface:

- type Spec struct { Pattern *regexp.Regexp; Name string }
- func Parse(r io.Reader, format string) ([]Spec, error)   // format "text" (default) or "json"
- func Apply(mods []model.Modification, specs []Spec) []model.Modification

Text format: lines "pattern => name". Trim spaces around "=>" and name. Anchoring rule (match original): if pattern starts with "^" compile verbatim; else compile "^" + pattern + "/" (path-prefix). Blank lines ignored.
JSON format: array of { "pattern": string, "name": string }; same anchoring applied to pattern.
Regex safety: reject patterns exceeding a length bound (e.g. > 1000 chars) or that fail to compile -> usage error (terr code "invalid_group", exit 2). Use Go regexp (RE2, linear time - no catastrophic backtracking, so length bound is the main guard).
Apply: for each mod, find FIRST spec whose Pattern matches mod.Entity (regexp.MatchString / FindString semantics = re-find, unanchored search but our patterns are anchored). If matched, set Entity = spec.Name; else DROP the mod. Preserve order of surviving mods.

Port fixtures: end_to_end/regex-layers-definition.txt, text-layers-definition.txt, regex-and-text-layers-definition.txt into testdata (GPL attribution).

TDD cases (group_test.go):

1. TestParse_Text_PrefixAnchor: "src/Features/Core => Core" compiles to ^src/Features/Core/ ; matches "src/Features/Core/x.cs", not "other/...".
2. TestParse_Text_RegexVerbatim: "^src/.*Tests\.cs$ => Tests" used verbatim.
3. TestParse_JSON: JSON array parses to equivalent Specs.
4. TestApply_FirstMatchWins: entity matching two specs -> first spec's name.
5. TestApply_DropsUnmatched: entity matching no spec is removed.
6. TestApply_RemapsEntity: matched entity's Entity replaced by group name; other fields intact.
7. TestParse_InvalidRegex: bad pattern -> invalid_group usage error (exit 2).
8. TestParse_OversizePattern: >bound -> usage error.
9. TestApply_PortedLayerDefs: the three ported layer-definition files produce expected groupings on a sample.

## Acceptance Criteria

- Text and JSON specs parse; anchoring matches the original; first-match remap; unmatched dropped; order preserved.
- Invalid/oversize patterns -> usage error (exit 2).
- Ported layer-definition fixtures produce expected results. All cases pass; make validate green.

## Notes

**2026-07-14T12:16:33Z**

Implemented src/internal/transform/group (group.go, group_test.go, testdata). Surface per design: Spec{Pattern *regexp.Regexp; Name}, Parse(io.Reader, format) ([]Spec, error) for text (default) and json, Apply(mods, specs) []Modification. Anchoring: ^-prefixed patterns verbatim, else `^<pattern>/` path-prefix. First-match remap; unmatched dropped; input order preserved; input slice not mutated. All malformed group input (missing => separator, empty pattern/name, oversize >1000 chars, uncompilable regex, malformed JSON, unknown format) -> terr ErrInvalidGroup code=invalid_group exit 2 (a definition is flag-supplied, so it is a usage error, unlike gitlog parse errors which are input/exit 3). RE2 = no backtracking, so the length cap is the only regex guard. Ported 3 layer-definition fixtures (GPL attribution in testdata/README.md) since .local/refs was empty. Unblocks cod-0xx4 (pipeline).
