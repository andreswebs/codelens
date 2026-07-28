---
id: cod-6xux
status: closed
deps: [cod-nve7]
links: []
created: 2026-07-28T16:48:28Z
type: chore
priority: 3
assignee: Andre Silva
tags: [codelens, spec-003, docs, maintenance]
---
# Record the Flint upgrade checklist beside the version pin

Record the Flint upgrade checklist wherever the version pin lives, so a future upgrader
finds it without reading the spec.

Spec: docs/specs/003-flint-adapter/plan.md section 5.1 (ticket 4 of four).

## Why a manual checklist rather than a test

Decision Q8: pin the version and re-verify by hand. That keeps JavaScript out of the test
suite, at the cost of not detecting drift until someone runs the list. The blast radius is
bounded because `flint_spec.py` has no Flint dependency at all: only the renderer and the
type and chart-type NAMES are exposed.

Item 6 exists because decision F12 accepts a COPY of Flint's declared channels inside
`flint_spec.py` that no unit test can verify, precisely because Q8 ruled out the Deno test
lane. `flint_render.ts` catches that drift at render time, but only for specs that are
actually rendered.

## Design

Put the checklist next to the pin, which after the renderer ticket means beside the
`npm:flint-chart-mcp@0.4.0/render` import, and reference it from the skill's
`references/`. A reader changing the pin must not have to find the spec.

## The six items

1. All 9 target semantic types still resolve: `Name`, `Category`, `Boolean`, `ID`, `Date`,
   `Count`, `Quantity`, `Duration`, `Percentage`.
2. `Network Graph` still reads `x` as source and `y` as target. Compile a two-column edge
   table and inspect `series[0].links`.
3. `_warnings` still carries overflow entries on the assembled spec. The truncation
   reporting depends on this PRIVATE field.
4. `intrinsicDomain` is still a GATE rather than an override. If this changes, the bare
   `percentage` mapping should be revisited, since the current mapping exists only
   because annotating it causes a 100x rendering error.
5. The 6 default chart-type names are unchanged.
6. The declared channel lists are unchanged, and `flint_spec.py`'s channel table still
   matches them. This is the only SCHEDULED parity check for that table.

## Why pin 0.4.0 rather than latest

Deno's default `minimumDependencyAge` guard refuses a version published under 24 hours
ago; verified against 0.4.1. Record that, so an upgrader who hits the error knows it is a
supply-chain default rather than a broken package, and does not reflexively disable it.

## Acceptance Criteria

- The six-item checklist recorded beside the version pin, discoverable without reading
  the spec.
- Referenced from the skill's `references/`.
- Explains why the pin is 0.4.0 and not latest, including Deno's `minimumDependencyAge`
  behaviour.
- Names item 6 as the only scheduled parity check for `flint_spec.py`'s channel table.
- markdownlint clean.


## Notes

**2026-07-28T19:02:38Z**

Six-item Flint upgrade checklist now lives directly above the pinned imports in docs/skills/codelens/scripts/flint_render.ts, replacing the previous three-line pointer at the spec. It is self-contained: the 9 semantic type names, the Network Graph x=source/y=target convention, the private _warnings field, intrinsicDomain as a gate (and why the bare percentage mapping depends on that), the 6 default chart-type names, and the declared channel lists. Item 6 is explicitly named as the only scheduled parity check for flint_spec.py's channel table. The 0.4.0-not-latest rationale (Deno minimumDependencyAge refuses versions under 24h old, verified against 0.4.1) is recorded there too, with the instruction to wait the window out rather than disable the guard. references/flint.md summarises the checklist and points at the script. Verified with markdownlint-cli2 (clean), deno check (clean), and make build (green). Nothing enforces the checklist in CI by design (Q8); the renderer's TEMPLATE_CHANNELS validation still catches channel drift at render time for rendered specs only.
