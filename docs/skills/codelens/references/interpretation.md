# Reading the crime scene

The interpretation authority for the codelens visualizations. Loaded from
[SKILL.md](../SKILL.md) step 5. The [catalog](catalog.md) `Read:` lines are hooks;
this file holds the full reading: the investigative funnel that orders an
investigation, a reading block per visualization, the heuristics table (the one
home for every number), and the guardrails the social analyses must respect.

The recurring picture is a **hotspot**: complicated code that changes often. An
investigation is offender profiling: narrow from where the risk is, to whether it
is getting worse, to why it spreads, to who it touches.

## The investigative funnel

Each stage narrows the search and feeds the next. Run them in order; later stages
only make sense once the earlier ones have pointed somewhere.

1. **Scope** (`summary`) — commits, entities, authors, and the window. One year is
   a good default.
2. **Where** (hotspots) — geographical profiling to a prioritized list of code to
   improve. The offender is problematic code.
3. **Worsening?** (complexity trend) — is a hotspot deteriorating, or already being
   refactored? Prefer the trend's shape over any absolute number.
4. **Why it ripples** (change coupling) — hotspots rarely walk alone; coupling
   exposes the implicit dependencies. Sum-of-coupling names the architecturally
   central modules; grouping by component surfaces coupling that crosses
   architectural boundaries.
5. **Who** (the social analyses) — ownership, fragmentation, and communication.
   Software is made of people; getting this wrong wrecks more codebases than bad
   code does.

Leading words to think with: **offender profile**, **geographical profiling**,
**power law**, **expected-vs-surprising coupling**, **sum-of-coupling**, **truck
factor**, **Conway litmus**, **Red/Yellow/Green**.

## Reading each visualization

### Hotspot map

Size is LOC (a complexity proxy); colour is change frequency. **Colour (change) is
the lead signal; size is the severity multiplier** — a large pale circle is
complex-but-stable and lower priority. Generated and vendored files are textbook
false positives (they change or bulk out for machine reasons, not because a
developer maintains them); scope them out before naming the top hotspot. The real
offender is the most-changed hand-maintained file; then run a complexity trend on
it. See the power-law and 1-5% / 25-75% rows in [Heuristics](#heuristics).

### Complexity trend

Indentation is the complexity proxy; the **shape** matters more than the number.
Three shapes: deteriorating (rising — act, refactor), refactored (a dip — good,
keep monitoring), stable (flat — fine). There is no "growing" shape: a rising line
is disambiguated by overlaying LOC — rising in step with LOC is growth by addition
(less alarming); the complexity line outpacing LOC is true structural
deterioration.

### Change-coupling graph

Edges are files or components that change together (degree = % of shared commits;
node weight = sum-of-coupling = architectural centrality). The signal is coupling
that is **surprising and crosses an architectural boundary**, not raw high degree —
a test and its implementation, or sibling forks, sit near 100% and are perfectly
benign. Coupling within a boundary is expected. Causes and their actions:
copy-paste (extract the shared unit), unsupportive module boundaries (co-locate
what changes together), producer-consumer (often legitimate — use domain
judgment). Prioritise couplings that are volatile and overlap hotspots. The tool
names suspects; a human reads the code to confirm the crime.

### Knowledge / ownership map

One colour per developer. A single-colour component is a key-person dependency; a
mixed one is shared effort. Mixed is not automatically healthy — the **degree** of
the main developer's ownership is the load-bearing signal, and key-person risk is
amplified where the code is also low quality. Phrase a finding as _who to talk to
and who the fallback is_, never as a productivity ranking (see
[Guardrails](#guardrails-for-the-social-analyses)).

### Code-age map

Read through **stabilization**, not calendar age. Most code should stabilize;
stable cores that have earned the right to be left alone are a virtue. The smell is
_old code that still churns_ — a low-cohesion signal (the module has many reasons
to change). There is no age threshold or "frozen" rule here; the map shows change
dynamics, and a stable periphery around a few churning cores is the healthy shape.

### Communication / Conway network

The Conway litmus test — but only after **aggregating individuals to teams**
(`--team-map`); the question is about organizational units, not people. Most
communication paths should be intra-team; paths that cross team boundaries are
_potential_ coordination bottlenecks (an occasional one is a healthy
helpful-colleague signal). When there is a bottleneck, the usual root cause is
technical (low cohesion in a shared module), so the fix is refactoring, not
reorganization. Resolve author aliases first, or a person links to their own alias.

### Fractal / fragmentation

Three ownership patterns predict quality: a single developer (consistent, but a
key-person risk), balanced contributors (higher main-developer ownership predicts
fewer defects), and many minor contributors (defect risk). When both signals are
present, lead with the minor-contributor pattern — the **count of minor
contributors** is the stronger defect predictor.

### Churn trend

Added-versus-deleted over time, at the project level. It is the macro backdrop for
the business case: sustained one-sided growth is accumulation, and trends make
waste visible far better than a snapshot.

### Commit word cloud

A **heuristic only** — a conversation starter, never a hard finding. Domain terms
dominating is healthy; bug / crash / revert / bump dominating is a cue to drill
deeper.

### Summary tiles

Scope and situational awareness (commits, entities, authors, window) — the framing
a report opens on.

## Heuristics

Every number lives here, once, as a rule of thumb rather than a law.

| Heuristic         | Rule of thumb                                                                                                                                                |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Hotspot / defects | ~1-5% of code holds ~25-75% of defects (one system: 4% of code, 72% of defects — the high end)                                                               |
| Defect prediction | change frequency predicts >75% of defects; it beats pure complexity                                                                                          |
| Analysis window   | one year default; ~one month for very high-churn repos; too much history flags cooled hotspots                                                               |
| Prioritization    | refactor down the ranked hotspot list; stop where the revision count levels off (power law)                                                                  |
| Coupling floor    | file level `--min-coupling 30`; at the component level use ~20, where 22-28% is already meaningful                                                           |
| Truck factor      | remove authors until >50% of files are abandoned; ~2/3 of projects are 1-2; ~41% survive the main dev leaving                                                |
| Business case     | healthy code ~124% faster to ship; unhealthy ~15x more defects and ~10x task-time variance; industry wastes 23-42% of dev time; ~15% baseline unplanned work |

## Guardrails for the social analyses

The ownership, fractal, and communication analyses describe **code and coordination
risk**, not people. Enforce these whenever a social analysis is read or reported:

- **Never rank or rate individuals.** These analyses are not a productivity summary
  and were not built to evaluate people (the fundamental attribution error and
  Goodhart's law both bite). Phrase findings as risk to the code, not judgments of
  a person.
- **Resolve author aliases first** (a repo `.mailmap`, or `--team-map` to a
  canonical identity), and exclude bulk-import commits that misattribute ownership.
- **Aggregate to teams or components** before the Conway reading.
- **Don't shoot the messenger** — the loudest symptom is rarely the root cause.
- **Everything is probabilistic** — hedge with risk language, not certainty.
- **Data does not replace talking to the team.**

## Communicating findings

When the audience is not the authoring team, rephrase the "why" in business terms:
time-to-market, customer satisfaction, roadmap risk. Avoid jargon (Red / Yellow /
Green reads better than a complexity score); present non-normalized human numbers
("2,500 lines", not "0.07"); prefer trends over snapshots. The presentation flow:
get attention (consequences in outcome terms) → build situational awareness (where
the strong and weak parts are) → focus on the vital few (debt in a hotspot is a
"payday loan") → set expectations. Close on the risk choice: accept, prioritise
low-risk work, or mitigate.
