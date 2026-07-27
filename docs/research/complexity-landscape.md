# Code Complexity Analysis and Its Fusion with Behavioral (Git-History) Analysis: State of the Art

Research briefing for extending codelens. Focus: how the market measures
complexity, how it fuses complexity with churn/coupling/ownership, and what the
evidence actually says about whether that fusion (or static complexity at all)
earns its place.

## 1. CodeScene Code Health: what actually feeds the 1-10 score

CodeScene's **Code Health** is the reference implementation of "complexity as a
compound of code smells, weighted by size, prioritized by behavior." Key facts
from their docs:

- The score runs **10 (healthy) down to 1 (severe)**. Files are bucketed:
  **healthy 9 or above, warning 4 to 9, alert below 4**.
- It aggregates **25 to 30+ factors** (the exact count depends on language; around
  31 languages supported). Most factors are **language-agnostic in definition,
  differing only in threshold values per language**.
- The file-level score is a **weighted average across functions, weighted by lines
  of code** per unit. Rules are configurable via a JSON template (set a rule
  weight to `0.0` to disable, or lower a weight to de-prioritize).

**The critical design idea: rules are compound.** Smells are not scored in
isolation; a finding fires when several primitive metrics co-occur. The canonical
example, a **Brain Method**, combines (a) lines of code, (b) cyclomatic
complexity, (c) deeply nested logic, and (d) a **centrality** score (how connected
the function is). This is what distinguishes it from a plain cyclomatic-complexity
linter.

The factors, grouped as CodeScene documents them:

| Category                  | Factors                                                                                                                                                                                                                    |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Module / class smells** | Low Cohesion (LCOM4), Brain Class / God Class, Developer Congestion (many devs concurrently), Complex Code by Former Contributors (knowledge loss)                                                                         |
| **Function smells**       | Brain Method / God Function, Complex Method (cyclomatic complexity), Large Method, DRY Violations (duplication that actually co-changes), Primitive Obsession                                                              |
| **Implementation smells** | Nested Complexity (ifs inside ifs/loops), Bumpy Road (multiple un-encapsulated logic chunks in one function), Complex Conditional (multi-operator AND/OR expressions), Large Assertion Blocks, Duplicated Assertion Blocks |

Configurable thresholds are exposed as knobs like
`function_cyclomatic_complexity_warning` and `function_nesting_depth_warning`.

**How complexity fuses with churn to prioritize.** Code Health measures _what the
code looks like_. It is layered on top of **Hotspots**, which measure _how you
work with the code_: by default the hotspot criterion is **commit frequency
(change frequency) of a file**. The product view CodeScene calls **Technical Debt
Friction** intersects the two: low Code Health x high development activity = the
files where poor quality actively slows the team, i.e. where refactoring pays off.
Debt in cold code is explicitly treated as "may be wasteful to fix."

At function/method granularity this is **X-Ray**, which within a hot file computes
per-method cyclomatic complexity, **change coupling between methods** (e.g. "these
two methods change together 42% of the time"), copy-paste detection _prioritized
by change coupling_ (clones that actually co-change), and a **Complexity Trend**
over time (is this method being refactored, or still degrading?).

- [Code Health docs (6.0.19)](https://docs.enterprise.codescene.io/versions/6.0.19/guides/technical/code-health.html)
- [CodeHealth product page](https://codescene.com/product/code-health)
- [Hotspots / Technical Debt docs (7.2.0)](https://docs.enterprise.codescene.io/versions/7.2.0/guides/technical/hotspots.html)
- [X-Ray docs](https://codescene.io/docs/guides/technical/xray.html)
- [Refactoring targets use case](https://codescene.com/use-cases/refactoring-targets)

**Empirical backing (Tornhill and Borg).** The "Code Red" study analyzed 39
proprietary codebases / 30,737 files, joining CodeScene analysis with Jira:
**low-quality (alert) code has around 15x more defects than healthy code, takes
around 124% longer time-in-development, and has around 9x longer worst-case cycle
times** (all differences significant at p=0.001). A 2024 follow-up ("Increasing,
not Diminishing") argues returns on maintainability are increasing, not
diminishing. A 2026 study claims **AI coding assistants raise defect risk by 30%
or more when applied to unhealthy code**.

- [Code Red (arXiv 2203.04374)](https://arxiv.org/pdf/2203.04374)
- [Increasing, not Diminishing (arXiv 2401.13407)](https://arxiv.org/html/2401.13407v1)
- [Technical Debt Friction industrial study (arXiv 2607.01850)](https://arxiv.org/pdf/2607.01850)

## 2. How mainstream tools measure and threshold complexity

| Tool                    | Primary complexity metric                                                                          | Default / recommended threshold                                                                | Notes                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **SonarQube**           | **Cognitive Complexity** (its own invention) is the flagship; Cyclomatic Complexity still reported | **15** per function (rule level); cyclomatic often warned above 20 at file level               | Cognitive penalizes nesting, break-in-flow, recursion; cyclomatic treats a 10-branch switch = 10 nested ifs. Best practice: enforce cognitive at method level via quality-profile **rules** (tag `brain-overload`), not as a project-wide quality-gate **measure**. Default "Sonar way" gate targets _new code_ (80%+ coverage, 3% or less dup, A ratings) and does not gate complexity directly. |
| **Code Climate / Qlty** | **Cognitive Complexity** (their "Complexity" = cognitive)                                          | file "complex" over configurable threshold; remediation time in minutes rolls into tech-debt   | Surfaces churn alongside complexity/duplication/coverage. Converts smells to remediation minutes to tech-debt.                                                                                                                                                                                                                                                                                    |
| **PMD**                 | **Cyclomatic Complexity**                                                                          | **method 10, class 80**; NPATH 200                                                             | Bands: 1 to 4 low, 5 to 7 moderate, 8 to 10 high, 11+ very high.                                                                                                                                                                                                                                                                                                                                  |
| **Checkstyle**          | **Cyclomatic Complexity** (methods/ctors/initializers only)                                        | **10** (also offers NPATH=200)                                                                 | Guidance: 10 is an aspirational target, keep below 20.                                                                                                                                                                                                                                                                                                                                            |
| **Codacy**              | **Cyclomatic Complexity**                                                                          | configurable "file is complex when over"; PR complexity sums file deltas where delta 4 or more | Rolls complexity + duplication + coverage + issues into a repo/file **grade**.                                                                                                                                                                                                                                                                                                                    |
| **CodeFactor**          | Issue density (incl. a "code complexity" issue class)                                              | **A-F grade**, no published formula                                                            | Grade = weighted issue-to-size ratio, **weighted by file change frequency/importance** (churn-weighted). VS Code: 686 issues / 3,133 files still = A.                                                                                                                                                                                                                                             |

The market split is clean: **IDE/linter-lineage tools (PMD, Checkstyle, Codacy)
center on cyclomatic complexity; the "code quality platform" generation
(SonarQube, Code Climate/Qlty, CodeScene) has moved to cognitive/comprehension-
oriented complexity** because cyclomatic conflates readable breadth (switch) with
unreadable depth (nesting).

- [SonarQube metric definitions](https://docs.sonarsource.com/sonarqube-server/10.8/user-guide/code-metrics/metrics-definition)
- [Sonar community: cognitive vs cyclomatic in gates vs rules](https://community.sonarsource.com/t/difference-between-cognitive-cyclomatic-complexity-quality-gate-and-rule/25558)
- [Qlty (Code Climate) maintainability metrics](https://docs.qlty.sh/cloud/maintainability/metrics)
- [PMD default cyclomatic discussion](https://github.com/pmd/pmd/discussions/3042)
- [Checkstyle CyclomaticComplexity](https://checkstyle.sourceforge.io/checks/metrics/cyclomaticcomplexity.html)
- [Codacy metrics](https://docs.codacy.com/faq/code-analysis/which-metrics-does-codacy-calculate/)
- [CodeFactor docs / glossary](https://docs.codefactor.io/bootcamp/glossary/)

## 3. The fusion: static complexity x behavioral analysis

The dominant fusion pattern, popularized by Tornhill's "behavioral code analysis,"
is the **2-D hotspot**: rank files by **complexity (or LOC/Code Health) x change
frequency (churn)**; the top-right quadrant (complex AND frequently changed) is
where defects and cost concentrate. Extensions:

- **Change/logical coupling (temporal coupling):** files or methods that change
  together despite no structural dependency. Foundational work (Gall et al.,
  release-history analysis of a telecom switch) established logical coupling; later
  research shows **change coupling predicts defects and is especially good at
  flagging severe/high-priority bugs**. CodeScene's Sum-of-Coupling (SOC) filters
  this to architecturally significant modules.
- **Ownership / socio-technical:** number of contributors, ownership dispersion,
  knowledge loss (former-contributor code). Feeds "developer congestion" and
  "knowledge loss" factors.
- **Architectural hotspot patterns** (Cai/Kazman/Mo/Xiao): unstable interface,
  modularity violation, unhealthy inheritance, cyclic dependency, detected by
  joining structure with revision history; they found strong positive correlation
  between architectural flaws and bug counts / change effort across 9 to 18
  projects.

**Evidence the fusion beats either input alone.** The strongest evidence is
indirect but consistent: (1) change frequency alone is "the single most important
metric for quality issues" (multiple studies cited by CodeScene); (2) complexity
alone correlates with defects but weakly once size is controlled; (3) the
intersection concentrates defects/cost far above either marginal (Code Red's 15x
defect and 124% time gaps are measured on Code Health, which is complexity-composed
but reported at file granularity and _prioritized_ by hotspots). The honest caveat:
rigorous head-to-head "hotspot (fused) vs churn-only" defect-prediction ablations
are thin, and the strongest behavioral-vs-structural comparisons (Section 4)
actually favor process metrics on their own.

- [On the Relationship Between Change Coupling and Software Defects](https://www.researchgate.net/publication/221200492_On_the_Relationship_Between_Change_Coupling_and_Software_Defects)
- [Tornhill: A Crystal Ball to Prioritize Technical Debt](https://laszlo.substack.com/p/interesting-content-adam-tornhill)

## 4. Counter-evidence: process/behavioral metrics beat structural complexity

This is the most important section for a behavioral tool and the case is strong.

**Rahman and Devanbu (ICSE 2013), "How, and Why, Process Metrics Are Better."**
Process metrics (churn, commits, developer count, prior changes) outperform
product/code metrics (complexity, LOC, OO metrics) for defect prediction. The
_why_ is the load-bearing insight:

1. **Stagnation.** Code metrics are largely **static across releases** while the
   code churns; they do not move, so they cannot track where risk migrates.
   Process metrics are dynamic and track actual evolution.
2. **Collinearity/redundancy.** Code metrics are **highly correlated with each
   other** (and with LOC), so a bag of complexity metrics carries little
   independent signal.
3. Bottom line: _defect-proneness tracks how code evolves more than what it looks
   like at a snapshot._

- [Rahman and Devanbu 2013 (draft PDF)](https://research.cs.queensu.ca/home/ahmed/home/teaching/CISC880/F17/papers/HowAndWhyProcessMetricsAreBetter.pdf)

**Majumder, Mody and Menzies (EMSE 2021 / ICSE 2022), "Revisiting Process versus
Product Metrics: a Large Scale Analysis."** Re-ran the question on **722,471
commits across 700 GitHub projects** and confirmed it at scale, with striking
margins (median): **process metrics recall 98% / AUC 95% vs product metrics recall
44% / AUC 54%.** Product metrics barely beat random (AUC 54%). Caveats they add:
metric-importance rankings are unstable when moving from small to large scale, and
at scale you should ensemble multiple models.

- [Revisiting Process vs Product Metrics (arXiv 2008.09569)](https://arxiv.org/abs/2008.09569)
- [Springer EMSE version](https://link.springer.com/article/10.1007/s10664-021-10068-4)

**repowise-bench (2025 to 2026)** is the notable counter-nuance from a commercial
deterministic scorer. It scores files 1 to 10 from **25 deterministic markers**
(McCabe complexity, brain methods, **LCOM4 cohesion**, god classes, Rabin-Karp
clones, untested hotspots, **change entropy, ownership, co-change, prior-defect
history**), weights learned from real defect history. Reported: files below 4.0
have **10 to 75x** more bug-fix commits than files above 8.0 (p<0.01);
cross-project mean **ROC AUC around 0.74, up to 0.90 in-repo**; external
PROMISE/jEdit AUC 0.76 to 0.78; and a head-to-head claiming **0.731 vs CodeScene
0.705**. The critical honest admission buried in it: **file size is a dominant
predictor**, within a single NLOC band, the residual signal is weak (**AUC around
0.49, i.e. chance**). That directly supports the "complexity is largely redundant
with LOC" thesis.

- [repowise-bench](https://github.com/repowise-dev/repowise-bench)
- [repowise README](https://github.com/repowise-dev/repowise/blob/main/README.md)
- [Code Health complete guide (repowise): "git history out-predicts code shape for defects"](https://www.repowise.dev/blog/guides/code-health-complete-guide)

**Balanced read, when does static complexity add signal beyond churn / beyond
LOC?**

- **Redundant with LOC:** raw cyclomatic complexity and most volume-ish complexity
  metrics are strongly collinear with LOC. Once you have file size and churn, they
  add little for _defect prediction_. The within-NLOC-band AUC around 0.49 is the
  cleanest single data point.
- **Adds signal:** (a) **cognitive/nesting-shaped** metrics (cognitive complexity,
  nested complexity, bumpy road) capture _comprehension_ difficulty that LOC
  misses, and better predict _maintenance effort / time-in-development_ than defect
  counts; (b) complexity is **actionable/localizing** in a way churn is not: churn
  tells you _where_ to look, complexity tells you _what to fix_ at method
  granularity; (c) **compound** signals (Brain Method = LOC + complexity + nesting
  - centrality) beat any single primitive; (d) **complexity trend** (rising over
    time) is a process-flavored use of a structural metric and is more informative
    than a static snapshot.

The synthesis the literature supports: **for predicting defects, behavioral/process
metrics dominate and static complexity is mostly redundant with size; for
prioritizing and guiding remediation (effort, comprehension, actionability),
comprehension-oriented complexity earns its keep.** Those are different jobs.

## 5. Newer angles (2024 to 2026)

- **LLM-based readability / cognitive-load assessment.** GPT-4o-as-judge for
  readability skews to **surface/syntactic features** (clarity mentioned 7.7%,
  complexity 9.9% of the time), missing deeper cognitive load. Human developers
  rate Claude-written code as more "developer-friendly" even when it has more
  errors. Practical implication: LLM judgments are not yet a reliable complexity
  oracle.
  - [Human-Aligned Code Readability Assessment with LLMs (arXiv 2510.16579)](https://arxiv.org/html/2510.16579v1)
  - [Evaluating LLM-Generated Code: benchmark + developer study](https://arxiv.org/html/2605.09059v1)
- **Traditional metrics fail on machine-generated code.** PMD Cognitive Complexity
  assigns **zero to more than 99% of LLM-generated test methods**; proposed
  replacements are readability-aware (CCTR) and cognition/behavior-based
  (NRevisit), the latter arguing (with neuroscience/biometric evidence) that
  **structural complexity alone does not reflect real comprehension difficulty**.
  - [Mind the Gap: Readability-Aware Test Code Complexity (CCTR)](https://www.researchgate.net/publication/392529997)
  - [NRevisit: Cognitive-Behavioral Metric for Understandability (arXiv 2504.18345)](https://arxiv.org/pdf/2504.18345)
- **AI-friendliness as a new lens.** CodeScene reframes Code Health as predicting
  AI-assistant success: unhealthy code means **30%+ higher defect risk from AI
  edits** and up to 45% more tokens burned. Code health is being pitched as a
  guardrail for AI coding agents (see their CodeHealth MCP server).
  - [Code for Machines / AI-friendliness PR](https://tools.prnewswire.com/en-us/live/20813/release/20260128EN71904)
  - [CodeHealth MCP Server](https://codescene.com/product/code-health-mcp)
- **Effort-aware ranking (EADP).** The maturing standard for evaluating any
  prioritizer: rank by predicted defect density, measure **PofB@20% LOC** (bugs
  found when inspecting top 20% of code). Recent work stresses **normalizing
  rankings by size** (so you are not just re-discovering that big files have bugs)
  and combining LOC with process features (Entropy, NDEV, EXP, the MMJC method).
  This is the right yardstick for a codelens hotspot ranker.
  - [On effort-aware metrics for defect prediction (EMSE)](https://link.springer.com/article/10.1007/s10664-022-10186-7)
  - Code churn: a neglected metric in effort-aware JIT defect prediction, Liu et
    al. ESEM 2017
- **Change-coupling-informed complexity / architectural debt.** "Technical Debt
  Friction" (2026) argues change coupling captures only one source of maintenance
  difficulty and proposes a finer-grained activity-x-maintainability signal.
  Cai/Kazman ATD patterns quantify and _predict future maintenance cost_ of
  architectural debt.
  - [Technical Debt Friction (arXiv 2607.01850)](https://arxiv.org/pdf/2607.01850)

## (a) Which tool uses which complexity metric as primary

| Tool                    | Primary complexity metric                                                                                        | Fuses with behavioral data?                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| **CodeScene**           | Compound code smells (Brain Method etc.) built from cyclomatic + nesting + centrality + LOC, to Code Health 1-10 | **Yes**, hotspots (change freq), change coupling, ownership, complexity trend; this is its core |
| **SonarQube**           | **Cognitive Complexity** (cyclomatic secondary)                                                                  | No (static/PR-scoped; no git-history mining)                                                    |
| **Code Climate / Qlty** | **Cognitive Complexity**                                                                                         | Partial, reports churn alongside, but scoring is static                                         |
| **PMD**                 | **Cyclomatic Complexity** (method 10 / class 80)                                                                 | No                                                                                              |
| **Checkstyle**          | **Cyclomatic Complexity** (10; NPATH 200)                                                                        | No                                                                                              |
| **Codacy**              | **Cyclomatic Complexity**, to repo grade                                                                         | No                                                                                              |
| **CodeFactor**          | Issue density incl. complexity, to A-F grade                                                                     | Partial, grade weighted by **file change frequency**                                            |
| **repowise**            | **25 deterministic markers** (McCabe, brain methods, LCOM4, clones) + git behavior, weights learned from defects | **Yes**, churn, entropy, ownership, co-change, prior defects, hotspots                          |

Pattern: **cyclomatic** = the linter generation; **cognitive** = the platform
generation; **compound + behavioral** = CodeScene and repowise, the two tools
whose primary claim is defect/effort prediction rather than style enforcement.

## (b) Synthesis: when is adding static complexity to a behavioral tool worth it?

**Worth adding when:**

1. **The job is prioritization + remediation guidance, not just defect
   prediction.** Churn/hotspots localize _where_; complexity (especially at
   function level) tells you _what to fix_. codelens already mines
   coupling/hotspots/ownership; complexity is the missing "what to fix" half.
2. **You use comprehension-shaped complexity, not volume-shaped.** Cognitive
   complexity, nesting depth, and "bumpy road" add signal beyond LOC; raw
   cyclomatic and Halstead-ish volume metrics are largely LOC in disguise.
3. **You make it compound and behavior-gated.** The CodeScene lesson: a Brain
   Method (LOC + complexity + nesting + centrality) inside a hotspot is far more
   informative than any single metric anywhere. Complexity earns its place as an
   _intersection_ with churn, not as a standalone leaderboard.
4. **You track complexity _trend_, not snapshot.** Rising complexity in a hot file
   is a process signal (fights the stagnation critique) and predicts degradation.
5. **You want an effort/comprehension proxy** (time-in-development, review load, AI
   -assistant safety), where Code Red-style evidence is strongest.

**Not worth it (or actively misleading) when:**

1. **The goal is pure file-level defect prediction and you already have churn +
   LOC + prior defects.** Rahman/Devanbu and the 700-project replication show
   process metrics dominate (AUC 95% vs 54%), and repowise's own within-NLOC-band
   AUC around 0.49 shows complexity adds close to nothing once size is controlled.
   Adding cyclomatic here is redundant and inflates confidence falsely.
2. **You would rank by complexity alone.** You would mostly rediscover "big files
   are complex," an effort-unaware, size-confounded ranking. Always evaluate with
   an **effort-aware metric (PofB@20% LOC), size-normalized.**
3. **On LLM/AI-generated or test code**, where standard cognitive/cyclomatic
   metrics are known to misfire (more than 99% zero-scored test methods).

**Recommendation for codelens.** Add complexity as a **comprehension-oriented,
function-level, compound signal that is intersected with existing behavioral
outputs (hotspots, change coupling, ownership)** and surfaced as a _trend_, not as
a standalone defect predictor. Keep churn/ownership/coupling as the primary
defect-risk backbone (the evidence favors them), and position complexity as the
actionable "what and where to refactor" layer plus an effort/comprehension proxy.
Validate any resulting ranker with size-normalized, effort-aware metrics rather
than raw correlation with defects, so you do not ship a dressed-up LOC ranker.
