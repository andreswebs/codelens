---
id: cod-9yuv
status: closed
deps: []
links: [cod-3wut]
created: 2026-07-15T10:01:20Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, interpretation]
---
# Skill: crime-scene interpretation reference (reading guidance)

Add an interpretation layer to the `codelens` visualization skill so a run does not
just render the ten crime-scene visualizations but reads them with the expertise
from the source method (Tornhill, *Your Code as a Crime Scene*, 2nd ed). This is a
documentation-only ticket: one new reference, plus rewiring of SKILL.md step 5 and
terse pointers from the catalog cards. All interpretation material below was mined
from the book against real-repo exemplars; the reference carries NO per-line
attribution (it is implicit that the whole reference derives from that one source)
and consolidates every number into a single heuristics table.

## Deliverable

Create `docs/skills/codelens/references/interpretation.md` as the single "reading
the crime scene" authority. Rewire `SKILL.md` step 5 ("Read the crime scene") to
point at it, and give each catalog card a one-line hook plus a pointer into it.

## Content for `interpretation.md`

### 1. The investigative funnel (method + leading words)

The book is a layered method; later stages build on earlier ones. Use this order as
the reference's backbone (it is also the report sequence in the sibling reporting
ticket):

- **Stage 0 scope** - `summary`; one year is a good default window.
- **Stage 1 WHERE** - hotspots = geographical profiling to a prioritized list of code
  to improve. The offender is problematic code.
- **Stage 2 WORSENING?** - complexity trend; prefer trends over absolute values.
- **Stage 3 WHY IT RIPPLES** - change coupling; "hotspots rarely walk alone";
  sum-of-coupling = architecturally significant modules; group by folder and look for
  coupling that crosses architectural boundaries.
- **Stage 4 WHO** - the social/organizational analyses; software is made of people.

Anchor the reference on these leading words so they accrue meaning in one place:
offender profile, geographical profiling, hotspot as home base, power law,
expected-vs-surprising coupling, sum-of-coupling, truck factor, Conway litmus,
Red/Yellow/Green.

### 2. Per-analysis reading blocks

Each block co-locates definition, how to read it, how to phrase the finding, and
caveats.

- **Hotspot map** - size = LOC (complexity proxy), colour = change frequency;
  **colour/change is the lead signal, size is the secondary severity multiplier**. A
  large pale circle is complex-but-stable (lower priority). Generated/vendored files
  are textbook false positives; scope them out before naming the top hotspot. The
  real offender is the most-changed hand-maintained file; then run a complexity trend
  on it.
- **Complexity trend** - indentation as complexity proxy; the SHAPE matters more than
  the number. Three shapes only: deteriorating (act/refactor), refactored (a dip =
  good, keep monitoring), stable (ok). No "growing" shape - a rising line is
  disambiguated by overlaying LOC (rises with LOC = growth by addition, less
  alarming; complexity climbs faster than LOC = true deterioration).
- **Change-coupling graph** - edges = files/components that co-change (degree = %
  shared commits; node weight = sum-of-coupling = architectural centrality). Coupling
  *within* a boundary is expected; coupling that is *surprising and crosses an
  architectural boundary* signals decay. Highest degree is NOT the signal (test and
  implementation, or sibling forks, sit near 100% and are benign). Causes -> actions:
  copy-paste (extract), unsupportive boundary (co-locate), producer-consumer (often
  legitimate - domain judgment). Prioritise volatile couplings overlapping hotspots.
  The tool names suspects; a human reads the code to confirm.
- **Knowledge / ownership map** - one colour per developer; single-colour component =
  key-person dependency, mixed = shared effort. Mixed is not automatically good - the
  *degree* of the main developer's ownership is the load-bearing signal, and
  key-person risk is amplified by low code quality. Phrase findings as "who to talk
  to + the fallback", never as a productivity ranking.
- **Code-age map** - reframed via the *stabilization* principle: stable cores are a
  virtue; *old code that still churns* is the smell (a low-cohesion signal, many
  reasons to change). Use NO age threshold or "frozen" language; the 2nd edition
  defines no code-age analysis or age metric, so the reading rests on stabilization,
  not a measured age rule.
- **Communication / Conway network** - the Conway litmus test. MUST aggregate
  individuals -> teams before the reading is valid. Most paths should be intra-team;
  inter-team paths are *potential* coordination bottlenecks (an occasional one is a
  healthy helpful-colleague signal). The usual fix is technical (cohesion), not
  reorganisation. Aliases must be resolved first.
- **Fractal / fragmentation** - three ownership patterns: single developer
  (consistent but key-person risk); balanced (higher main-dev ownership predicts
  fewer defects); many minor contributors (defect risk). The *count of minor
  contributors* is the STRONGER defect predictor - lead with it when both are present.
- **Churn trend** - project-level added-vs-deleted over time; the macro backdrop for
  the business/"unplanned work" argument. Trends make waste obvious.
- **Commit word cloud** - heuristic only, a conversation-starter, never a hard
  finding. Domain terms = healthy; bug/crash/revert/bump = drill deeper.
- **Summary tiles** - scope / situational-awareness framing.

### 3. Heuristics table (single source of truth for every number)

Put ALL numeric rules of thumb here, once, phrased as heuristics (not laws):

- Hotspots are typically **1-5% of code but hold 25-75% of defects** (one cited
  system: 4% of code, 72% of defects - the high end).
- A change-based defect model predicts **>75% of defects** (beats pure complexity).
- Analysis window: **one year default**; drop toward one month for very high-churn
  repos; too much history flags cooled-down hotspots.
- Power law: **refactor down the ranked hotspot list, stop where the revision count
  levels off**; you need not fix all debt.
- Change coupling: default `--min-coupling 30` is a FILE-level floor; at the
  COMPONENT level use **~20**, where **22-28% is already meaningful**.
- Truck factor: greedily **remove authors until >50% of files are abandoned**;
  **~two-thirds of projects have a truck factor of 1-2**; only **~41%** survive the
  main developer leaving.
- Business case: healthy code ships features **~124% faster**; unhealthy code has
  **~15x more defects**; its task time varies by **almost an order of magnitude**;
  industry wastes **23-42%** of developer time on debt; baseline unplanned work for
  high performers **~15%**.

### 4. Misuse guardrails (prominent section; enforced by the reporting ticket)

- **Never rank or rate individuals.** The social analyses are not a productivity
  summary and were not built to evaluate people (fundamental attribution error;
  Goodhart's law). Phrase everything as code/coordination *risk*.
- **Resolve author aliases first** (`.mailmap` / `--team-map`); exclude bulk-import
  commits that distort ownership.
- **Aggregate people to teams/components** for the Conway reading.
- **Don't shoot the messenger** - the loudest symptom is rarely the root cause.
- **Everything is probabilistic** - hedge with risk language, not certainty.
- **Data doesn't replace talking to the team.**

### 5. Communicating findings (report framing; brief, points to the reporting ticket)

Rephrase the "why" in business terms (time-to-market, customer satisfaction,
roadmap risk); avoid jargon (Red/Yellow/Green); present non-normalized human numbers;
prefer trends. The four-step flow: get attention -> situational awareness -> focus on
the vital few (hotspot debt = "payday loan") -> set expectations. Close on risk
choices: accept / prioritise low-risk / mitigate.

## Wiring changes

- `SKILL.md` step 5 ("Read the crime scene"): replace the inline one-liner guidance
  with a pointer to `references/interpretation.md`; keep the step's completion
  criterion ("the finding is named, not just the chart handed over").
- `references/catalog.md`: each card keeps a terse `Read:` hook and gains a pointer
  into `interpretation.md`. Update the one-liners that the book refines: hotspot
  (colour leads over size), coupling (surprise-not-degree; node weight = SOC),
  knowledge (main-dev ownership degree is the real signal), code-age (stabilization
  reframe, drop age-threshold language), communication (aggregate to teams first),
  fractal (minor-contributor count is the stronger predictor).

## Coordinate with `cod-3wut` (linked)

`cod-3wut` (docs) also edits the code-age catalog card and adds a code-age caveat.
Amend that caveat so it says the code-age reading rests on the stabilization
principle (the 2nd edition defines no code-age analysis or age threshold), in
addition to the window-capping warning already there. Keep the two tickets'
code-age wording consistent; land whichever is ready first and reconcile the other.

## Out of scope

- No changes to `codelens` (the CLI) or to any script. Interpretation is
  agent-applied guidance, consistent with the skill's determinism boundary (scripts
  pin transforms/renders; the agent picks and interprets).

## Acceptance criteria

- `references/interpretation.md` exists and is the single home for the funnel, the
  per-analysis reading blocks, the one heuristics table, and the misuse guardrails.
- `SKILL.md` step 5 points at it; every catalog card has a hook + pointer; the
  refined one-liners match this ticket.
- The code-age reading uses the stabilization framing with no age threshold and is
  consistent with `cod-3wut`.
- No per-line source attribution; numbers live only in the heuristics table.
- All edited Markdown passes `markdownlint-cli2 --config ~/.markdownlint.yaml`
  (project standard; no repo-local config).

## References

- `docs/skills/codelens/SKILL.md` (step 5, pipeline), `references/catalog.md`,
  `references/operating.md`
- Related: `cod-3wut` (code-age + catalog docs), the sibling reporting ticket
  (consumes this reference for findings), the degraded-renderers ticket
- Skills: `/llm-coding` (surgical, single source of truth)

## Notes

**2026-07-15T10:11:45Z**

Implemented. New references/interpretation.md is the single reading authority: investigative funnel + leading words, a reading block per visualization, one heuristics table (all numbers, no per-line attribution), the social misuse guardrails, and a communicating-findings section. SKILL.md step 5 rewired to point at it. catalog.md: added a top-of-file pointer (covers every card) and refined the Read: hooks the book corrects - hotspot (colour/change leads over size), coupling (surprising cross-boundary, not raw degree; node weight = SOC), knowledge (main-dev ownership degree is the signal), code-age (stabilization reframe, dropped the 'frozen' rule), communication (aggregate to teams first), fractal (minor-contributor count is the stronger predictor), complexity (LOC overlay), word cloud (heuristic only). cod-3wut already closed (its full-history code-age caveat is present and consistent). Numbers live only in the heuristics table. markdownlint clean; skill_ref validate passes.
