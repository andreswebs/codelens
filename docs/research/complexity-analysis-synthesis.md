# Complexity Analysis for codelens: Synthesis

This synthesizes three research reports into what code-complexity analysis is,
how it is computed, what the evidence says, and how it could fit codelens
(a read-only, git-log-mining, polyglot, agent-first evolutionary-analysis tool).
The underlying reports:

- [complexity-metrics-theory.md](complexity-metrics-theory.md), every meaningful
  metric, exact formulas, defect-prediction evidence, LOC-correlation critiques.
- [complexity-implementation.md](complexity-implementation.md), parser-based vs
  heuristic vs indentation, Go-ecosystem feasibility, computing over git history.
- [complexity-landscape.md](complexity-landscape.md), what CodeScene / SonarQube /
  Code Climate measure, and the process-vs-structural defect-prediction evidence.

## How complexity analysis actually works: three families

There is no free, universal, accurate complexity metric. Every accurate metric
needs per-language syntax knowledge; every language-agnostic metric is a proxy.
The field splits into three implementation families:

1. **Parser-based (accurate, per-language).** Build a syntax tree, then walk it
   counting constructs. The only way to get true cyclomatic or cognitive
   complexity. The realistic polyglot path is tree-sitter (100+ grammars, error
   recovery, incremental), but its queries are not portable across grammars, so
   "count the branches" is always a per-language node table you maintain. In Go it
   means CGO plus shipping N C grammars.
2. **Token/keyword heuristic (approximate, broad).** Do not parse; tokenize and
   count decision keywords (`if for while case catch && || ?`). This is what lizard
   (around 20 languages, per-function) and scc (200+ languages, whole-file, pure
   Go) do. Good enough to rank files within one language; misses recursion. scc
   being pure Go makes its scanner and language tables directly vendorable.
3. **Indentation / whitespace proxy (cheapest, universal, parser-free).** Strip
   blanks and comments, normalize tabs to spaces, measure indentation depth (and
   its moments / entropy) per line. Hindle et al. showed whitespace alone
   rank-correlates with McCabe and Halstead. Language-neutral, a few dozen lines of
   Go, no dependencies. It is what Tornhill's own indent-complexity-proxy uses, and
   what the codelens viz skill already prototypes in `complexity_trend.py`.

## The metrics that matter (evidence-ranked)

- **Cyclomatic (McCabe):** independent paths, decisions + 1. Ubiquitous, a genuine
  test-effort bound, but blind to nesting and largely redundant with LOC for defect
  prediction.
- **Cognitive Complexity (SonarSource):** built for understandability, increments
  on each break in linear flow, adds a surcharge for nesting depth, counts a whole
  switch as 1. The metric the modern platform generation moved to; the only
  structural metric with even moderate validation against human comprehension time.
- **Nesting depth:** one of the very few signals that correlates with defects
  beyond LOC. Cheap, approximable from indentation alone.
- **Halstead, Maintainability Index:** classic, but MI is unreproducible magic-
  coefficient 1990s regression, and Halstead is tokenizer-sensitive. Low priority.
- **OO coupling-cohesion (CBO, WMC, RFC, LCOM4):** best-validated design-level
  fault predictors, but need symbol resolution and call graphs, so not language-
  agnostic.
- **LOC:** the yardstick every other metric must beat, and mostly does not.

## The uncomfortable core finding

**Process/behavioral metrics beat static complexity for defect prediction,
decisively.** Rahman and Devanbu, and a 700-project, 722k-commit replication,
found process metrics at around 95% AUC vs product/complexity metrics at around
54% (barely above random). The reasons: code metrics are static across releases
while risk migrates with churn, and they are collinear with each other and with
LOC. Even repowise's own commercial scorer admits that within a single file-size
band, complexity's residual predictive power is AUC around 0.49 (chance).

So static complexity, added naively, risks shipping a dressed-up LOC ranker. But
it earns its place for three different jobs:

1. **Actionability.** Churn tells you _where_ to look; complexity tells you _what
   to fix_, at function granularity. codelens already has the "where" (hotspots,
   coupling, ownership); complexity is the missing "what."
2. **Comprehension / effort, not defects.** The Code Red study (39 codebases)
   shows low code health means around 15x more defects and around 124% longer
   time-in-development; the evidence is strongest for complexity predicting _effort
   and comprehension load_, which matters most for legacy navigation.
3. **Trend, not snapshot.** Rising complexity in a hot file is a _process_ signal
   (it moves), which sidesteps the stagnation critique. CodeScene's whole
   "complexity trend" feature is this.

CodeScene's design lesson: Code Health (1-10) is **compound and behavior-gated**.
A "Brain Method" is not high cyclomatic alone; it is LOC + cyclomatic + nesting +
centrality, and it only matters inside a hotspot. Complexity as an _intersection_
with churn, never a standalone leaderboard.

## What this means for codelens: a layered path

Tagged by boundary-flex and effort.

- **Lane 0, the enabling flex (decide first).** Today codelens only reads a git
  log. Any complexity metric requires reading **file content at revisions**.
  Cleanest option: pure-Go go-git (no subprocess, preserves "never runs git");
  fallback `git cat-file --batch --buffer`. Critical scaling trick regardless of
  lane: **cache complexity by blob SHA**, so each unique file version is measured
  once, not once per commit. This decision unlocks everything below.
- **Lane 1, indentation complexity + trend (recommended first cut).** Parser-free,
  universal, around 1 day, no CGO. Compute logical indentation per entity, and
  crucially _over history_ so you get the trend (rising, dipping, flat) paired with
  LOC. Matches codelens's evolutionary identity and is a process-flavored use of a
  structural metric, which the evidence favors. Boundary flex: content-reading
  only.
- **Lane 2, a real hotspot / code-health command (the fusion).** Promote the churn
  x complexity crossing (today done by hand in the viz skill) into a first-class
  `hotspot` analysis: rank by change frequency intersected with complexity,
  size-normalized, validated with an effort-aware yardstick (PofB@20% LOC) rather
  than raw defect correlation. Optionally a compound "brain method / brain file"
  flag. Boundary flex: opinionated scoring (keep weights transparent and
  configurable, since the ADR philosophy dislikes magic numbers).
- **Lane 3, keyword-counting complexity (broad, medium effort).** Vendor scc's
  pure-Go scanner + language tables for a defensible "cyclomatic-ish" per-file
  number across 200+ languages, no CGO. Label honestly as an approximation,
  comparable only within a language. Boundary flex: content-reading + a vendored
  dependency.
- **Lane 4, tree-sitter accurate lane (optional, gated).** True per-function
  cyclomatic + cognitive across languages, behind a build tag. Buys accuracy at the
  cost of CGO, shipping grammars, per-grammar node tables, heavier binaries. Only
  if users demand method-level precision. Boundary flex: largest.

## The decisions to tee up

1. **Do we let codelens read file content?** (Lane 0) The one real philosophical
   boundary; everything complexity depends on it. go-git keeps the "no subprocess"
   spirit.
2. **Complexity as trend/intersection, or as a standalone metric?** The evidence
   strongly favors trend + intersection with churn; a standalone complexity
   leaderboard would mostly re-rank by size.
3. **How far up the accuracy ladder?** Indentation (universal, cheap, trend-
   oriented) is almost certainly the right first rung. tree-sitter is a real
   product bet with a CGO tax, not a free upgrade.
4. **Comprehension-shaped, not volume-shaped.** If we add complexity,
   cognitive/nesting-style beats cyclomatic/Halstead, which are LOC in disguise.

Recommended first cut: Lane 0 (go-git content reading, blob-SHA cache) plus Lane 1
(indentation complexity trend) plus Lane 2 (`hotspot` fusion command). It makes
codelens self-contained (no tokei sidecar needed), plays to its evolutionary
strength via trends, and avoids the "dressed-up LOC ranker" trap by keeping churn
as the backbone and complexity as the actionable, comprehension-oriented layer.
