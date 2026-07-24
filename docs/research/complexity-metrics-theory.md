# Source-Code Complexity Metrics: A Theory and Evidence Report

Scope: every meaningful *code* complexity metric (structural, textual,
object-oriented, and readability/cognitive-load), with origin, exact
computation, distinctive signal, weaknesses, and empirical evidence on
defect/maintenance prediction, including the recurring critique that most
correlate with raw lines of code. Ends with a ranked shortlist for a
language-agnostic git-history tool.

## 1. Cyclomatic Complexity (McCabe)

**Origin.** Thomas J. McCabe Sr., "A Complexity Measure," *IEEE Transactions on
Software Engineering* SE-2(4), Dec 1976. Original paper:
[literateprogramming.com/mccabe.pdf](http://literateprogramming.com/mccabe.pdf).

**What it measures.** The number of *linearly independent paths* through a
function, computed from its control-flow graph (CFG). It is graph-theoretic: the
cyclomatic number (a Betti number) of the CFG.

**Formula.**

- `M = E - N + 2P` where E = edges, N = nodes, P = connected components (P = 1
  for a single subroutine, giving `M = E - N + 2`).
- Strongly-connected variant (McCabe's alternate framing): connect each exit back
  to entry, then `M` equals the graph's cyclomatic number.
- Practical single-entry/single-exit form: `M = D + 1`, where D = number of
  decision points (`if`, `else if`, `case`, `for`, `while`, `&&`, `||`, `?:`,
  `catch`). Each decision adds 1.

**What it captures that others miss.** A rigorous, language-agnostic count of
independent paths; gives a lower bound on the number of test cases for branch
coverage. NIST later recommended a max of 10 per function.

**Weaknesses / criticisms.**

- Does *not* account for nesting depth: a flat sequence of 10 `if`s scores the
  same as 10 deeply nested ones, though the latter is far harder to understand.
  ([Cyclomatic complexity: the nesting problem](https://www.researchgate.net/publication/261455470_Cyclomatic_complexity_The_nesting_problem))
- Treats each `switch`/`case` as a decision, over-penalizing readable multi-way
  branching.
- Can *increase* when good structuring rules are applied (Evangelist's
  observation, via Shepperd).
- Not actionable / hard to interpret; high values do not reliably mean low
  readability.

**Empirical evidence.** Martin Shepperd's foundational critique
([A critique of cyclomatic complexity as a software metric](https://www.cs.du.edu/~snarayan/sada/teaching/COMP3705/lecture/p1/cycl-1.pdf),
1988) argued v(G) "may well be no more than a proxy" for LOC and that its ability
to predict error rates/effort is "quite erratic." The strongest positive result
he cites is Henry et al.'s UNIX study (165 procedures) showing strong
CC-vs-error correlation. Later work found CC and executable LOC correlated at
R-squared around 0.93 in a mature system (`XLOC = 3.96 x STCYC`), reinforcing
redundancy ([defect patterns study, arXiv:1912.04014](https://arxiv.org/pdf/1912.04014)).
A controlled experiment found ELOC, Halstead Volume, and CC mutually correlated
at mean 0.904, and that all metrics together explained only 27.6% of defect-rate
variance ([Study on Correlations Between Program Metrics and Defect Rate](https://scialert.net/fulltext/?doi=jse.2013.114.120)).
A large-scale reassessment on 17.6M Java methods / 6.3M C functions found the
CC-SLOC linear correlation is only *moderate* once variance is accounted for,
complicating the simple redundancy narrative
([Landman et al., "CC and LOC: Empirical Evidence of a Stable Linear Relationship"](https://www.researchgate.net/publication/220204439_Cyclomatic_Complexity_and_Lines_of_Code_Empirical_Evidence_of_a_Stable_Linear_Relationship)).

**Related sub-metric, essential complexity:** McCabe's ev(G) measures
unstructuredness by reducing the CFG of well-structured (D-structured) primes;
ev(G) > 1 signals unstructured control flow (spaghetti).
([Essential complexity, Wikipedia](https://en.wikipedia.org/wiki/Essential_complexity))

## 2. Cognitive Complexity (SonarSource)

**Origin.** G. Ann Campbell, SonarSource: "Cognitive Complexity: A new way of
measuring understandability" (whitepaper first published around 2016 to 2018;
current rev. 2023). PDF:
[sonarsource.com/docs/CognitiveComplexity.pdf](https://www.sonarsource.com/docs/CognitiveComplexity.pdf).
Blog:
[Cognitive Complexity, Because Testability != Understandability](https://www.sonarsource.com/blog/cognitive-complexity-because-testability-understandability/).

**Purpose.** Explicitly built to measure *understandability/maintainability*
rather than testability, remedying CC's shortcomings. It breaks from a pure
mathematical model in favor of a human-assessment-driven scheme.

**The three guiding rules.**

1. **Increment** for each break in the linear (top-to-bottom, left-to-right)
   reading flow.
2. **Increment (extra)** when flow-breaking structures are nested.
3. **Ignore** "shorthand" structures that condense multiple lines readably
   (notably method calls).

**Four increment categories** (the type is only conceptual; each increment is +1
to the score):

- **Structural** - control-flow structures subject to nesting increment.
- **Fundamental** - statements *not* subject to nesting increment.
- **Hybrid** - control-flow structures not subject to nesting increment but which
  *do* raise the nesting level.
- **Nesting** - the per-level surcharge.

**Exact increment rules.**

- **Structural increment (+1, and +nesting-level):** `if`, ternary `?:`, `switch`
  (once for the whole switch, cases and all), `for`, `while`, `do...while`,
  `catch`.
- **Hybrid increment (+1, no nesting surcharge, but increases nesting level for
  children):** `else if`, `else`, `elif` (the mental cost of the condition was
  already paid at the `if`).
- **Fundamental increment (+1, no nesting surcharge):** `goto`/`break`/`continue`
  to a *label*; each **sequence of like binary boolean operators**; recursion.
- **Nesting level is incremented by:** loops (`for`/`while`/`do`), conditionals
  (`if`, ternary), `switch`, `catch`, and nested function/lambda definitions.

**Boolean operator sequences (precise rule).** Cognitive Complexity does *not*
increment per `&&`/`||`. It adds +1 for each **new sequence of like operators**;
switching operator type starts a new sequence. Worked example from the
whitepaper:

```text
if (a          // +1 (if)
    && b && c  // +1 (one && sequence)
    || d || e  // +1 (new || sequence)
    && f)      // +1 (new && sequence)
```

Negation forcing a new inner sequence, e.g. `if (a && !(b && c))`, scores +1
(if) +1 (`&&`) +1 (inner `&&`).
([community.sonarsource.com discussion](https://community.sonarsource.com/t/inconsistent-cognitive-complexity-calculation-for-logical-operators/136166);
[clang-tidy readability-function-cognitive-complexity](https://clang.llvm.org/extra/clang-tidy/checks/readability/function-cognitive-complexity.html))

**Recursion:** each recursive call adds +1 (a cycle the reader must reason about,
which CC ignores).

**Worked totals from the whitepaper.**

- `sumOfPrimes` (nested for to for to if): first `for` +1, nested `for` +2 (1+1
  nesting), nested `if` +3 (1+2 nesting), so **Cognitive = 7** vs
  **Cyclomatic = 4**.
- `getWords` (a `switch` with several cases): **Cognitive = 1** vs
  **Cyclomatic = 4**.
- Methods themselves add +0, so plain data classes score 0 regardless of method
  count.

**What it captures that others miss.** Nesting depth, mixed-boolean difficulty,
and the readability benefit of decomposition (method calls are free), none of
which CC rewards.

**Weaknesses / criticisms.** Language-implementation edge cases produce
inconsistencies; the negation/boolean rules confuse developers; still a static
structural proxy, not validated ground truth.

**Empirical evidence.** Munoz Baron, Wyrich and Wagner, "An Empirical Validation
of Cognitive Complexity as a Measure of Source Code Understandability," ESEM 2020
([arXiv:2007.12520](https://arxiv.org/pdf/2007.12520)) pooled data from published
understandability studies and found *moderate but statistically significant*
correlations with comprehension time and subjective ratings, the first metric
with such validation. However, Lavazza et al. (JSS 2022,
[ScienceDirect](https://www.sciencedirect.com/science/article/abs/pii/S0164121222002370))
reused that data and concluded Cognitive Complexity correlates with
understandability "approximately as much as traditional measures," so it "does
not appear to fulfill the promise of being a significant improvement."

## 3. Halstead Complexity Suite

**Origin.** Maurice H. Halstead, *Elements of Software Science*, 1977. Reference:
[Halstead complexity measures (Wikipedia)](https://en.wikipedia.org/wiki/Halstead_complexity_measures);
[Verifysoft Halstead Metrics](https://www.verifysoft.com/en_halstead_metrics.html).

**Base counts.** Tokens are classified as operators or operands:

- `n1` = distinct operators, `n2` = distinct operands
- `N1` = total operators, `N2` = total operands

**Derived measures.**

- Vocabulary: `n = n1 + n2`
- Length: `N = N1 + N2`
- Calculated length: `N' = n1*log2(n1) + n2*log2(n2)`
- **Volume:** `V = N * log2(n)` (information content in bits; guideline 20 to
  1000 per function)
- **Difficulty:** `D = (n1 / 2) * (N2 / n2)`
- **Effort:** `E = D * V`
- Time to program: `T = E / 18` seconds (Stroud number S = 18)
- Delivered bugs: `B = V / 3000`

**What it captures that others miss.** Focuses on *vocabulary/information
content* rather than control flow; less sensitive to layout than LOC; captures
operand/operator density.

**Weaknesses / criticisms.**

- Highly dependent on correctly classifying operators vs operands, no
  cross-language consensus, so results vary by tokenizer.
- Effort/time equations rest on Stroud's psychological "mental discriminations
  per second," criticized as a poor cognitive model.
- Empirical support is thin beyond length estimation, and length prediction is
  circular (needs N1, N2 from the code itself). Shepperd and Ince: developed in
  the batch era for programs of a few hundred LOC.
  ([An Analysis of the Design and Definitions of Halstead's Metrics](https://www.researchgate.net/publication/260843757_An_Analysis_of_the_Design_and_Definitions_of_Halstead's_Metrics))
- Very low practitioner adoption vs LOC/CC.

**Empirical evidence.** Mixed. A controlled study found Halstead E, CC, and
length all correlated with modification accuracy/time, but *primarily in
unstructured, uncommented code and for less-experienced programmers*
([Measuring the Psychological Complexity of Software Maintenance Tasks](https://ieeexplore.ieee.org/document/1702603/)).
A recent study found Halstead Effort/Difficulty correlate with cognitive load
*better* than cyclomatic or cognitive complexity, positioning it as a useful
complement. Decomposed Halstead metrics have shown value in fault prediction
([PeerJ, decomposed Halstead in fault prediction](https://peerj.com/articles/cs-1647/)).
The controlled-experiment study above found only Halstead Volume (with nesting
depth and number of procedures) moderately correlated (around 0.3) with defect
rate.

## 4. Maintainability Index (MI)

**Origin.** Paul Oman and Jack Hagemeister, University of Idaho: ICSM 1992
("Metrics for assessing a software system's maintainability," pp. 337 to 344) and
JSS 24(3), 1994 ("Construction and testing of polynomials predicting software
maintainability"). Derived by regressing 40 candidate metrics against expert
maintainability ratings (1 to 100) of HP C/Pascal systems. References:
[NDepend, Maintainability Index](https://blog.ndepend.com/maintainability-index/);
[Arie van Deursen, "Think Twice Before Using the Maintainability Index"](https://avandeursen.com/2014/08/29/think-twice-before-using-the-maintainability-index/);
[Microsoft Learn, MI range and meaning](https://learn.microsoft.com/en-us/visualstudio/code-quality/code-metrics-maintainability-index-range-and-meaning?view=visualstudio).

**Original 3-metric formula (unbounded, ranges from 171 down to negative):**

```text
MI = 171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)
```

where HV = Halstead Volume, CC = cyclomatic complexity, LOC = lines of code (all
averaged per module).

**4-metric variant** adds a comment term:

```text
MI = 171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC) + 50*sin(sqrt(2.4*perCM))
```

where perCM = fraction of comment lines. Criticized because large comment blocks
can inflate the score.

**A common statement-based variant** replaces LOC with average statement count
(aveSTAT), argued to reflect size better.

**Microsoft / Visual Studio variant (2011, bounded 0 to 100, comment term
dropped):**

```text
MI_VS = MAX(0, (171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)) * 100 / 171)
```

Rescaled so negatives collapse to 0. VS thresholds are deliberately conservative
(0 to 9 red, 10 to 19 yellow, 20 or more green, i.e. 20 counts as "good").

**Weaknesses / criticisms.** van Deursen's widely-cited critique: the magic
coefficients derive from tiny 1990s HP systems and were never re-calibrated; the
constants are unexplained; averaging hides outliers; MI is largely a repackaging
of its inputs (which are themselves LOC-correlated). Different tools implement
different variants, so scores are not comparable.

**Empirical evidence.** Recent benchmarks find MI a weak-to-mediocre proxy for
human maintainability judgments
([Ghost Echoes Revealed, arXiv:2408.10754](https://arxiv.org/pdf/2408.10754)); an
MDPI 2023 study of MI variants in OO systems found substantial disagreement
between variants
([Exploring Maintainability Index Variants](https://www.mdpi.com/2076-3417/13/5/2972)).

## 5. NPATH Complexity

**Origin.** Brian A. Nejmeh, "NPATH: A measure of execution path complexity and
its applications," *Communications of the ACM* 31(2):188 to 200, 1988.
[ACM DL](https://dl.acm.org/doi/10.1145/42372.42379).

**What it measures.** The number of *acyclic execution paths* through a function,
explicitly designed to fix CC's blindness to nesting. Unlike CC (which keeps
loops and counts basic paths), NPATH eliminates loops and counts distinct acyclic
paths, so it grows *multiplicatively* with nesting rather than additively.

**Algorithm (per-construct expressions, composed multiplicatively for
sequences):**

- Sequence of statements: product of their NP values.
- `if`: `NP(if) = NP(else-branch or 1) + NP(then) + expr`; roughly the branch
  paths sum, times nested content.
- `if...else`: `NP(then) + NP(else) + NP(expr)`
- `while` / `do...while`: `NP(body) + NP(expr) + 1`
- `for`: `NP(body) + NP(expr) + 1`
- `switch`: sum over cases of `NP(case-body)` + `NP(expr)`
- Logical expression: `NP(expr) = number of && and || operators`
- `return expr`: `NP(expr)`
- A straight-line function of two nested `if`s gives 2 x 2 = 4 paths, capturing
  the combinatorial blow-up CC misses.

**Weaknesses / criticisms.** Nejmeh's C definition does not actually count acyclic
paths correctly even for simple programs; `continue` (a back-edge) is unhandled;
exact acyclic-path counting on a general graph is NP-complete. The later
**ACPATH** metric ([arXiv:1610.07914](https://arxiv.org/pdf/1610.07914)) gives a
fast, provably-exact estimate for MISRA-compliant C. Values explode into the
millions, making thresholds awkward.

**Empirical evidence.** Sparse; mostly used as a testability upper bound (paths to
exercise) rather than a validated defect predictor.

## 6. Nesting Depth / Maximum Indentation

**Origin.** Not attributable to a single paper; formalized in various nesting
studies, e.g., Alrasheed et al., "Measuring nesting," *IET Software* 2022
([Wiley](https://ietresearch.onlinelibrary.wiley.com/doi/full/10.1049/sfw2.12069)).

**What it measures.** Maximum (or average) depth of encapsulated scopes/blocks in
a function body, "how deeply nested" branches are, orthogonal to *how many*
branches (CC's territory).

**Computation.** Track block/brace/indentation depth during a single pass; record
the max depth reached. Language-agnostic when approximated via indentation.

**What it captures that others miss.** Reading deeply nested code forces the
reader to hold outer-block context on a mental stack; the difficulty rises
super-linearly with depth, a signal CC explicitly does not encode.

**Empirical evidence.** In the defect-rate controlled experiment, **nesting
depth** was one of only three metrics (with Halstead Volume and number of
procedures) moderately correlated (around 0.3) with defect rate and "one of the
most important dimensions that account for defect variability"
([scialert study](https://scialert.net/fulltext/?doi=jse.2013.114.120)). Used as
a distinct feature in defect-prediction models (max nesting depth) alongside CC
([Too Trivial To Test?, arXiv:1811.00820](https://arxiv.org/pdf/1811.00820)).

## 7. ABC Metric

**Origin.** Jerry Fitzpatrick, "Applying the ABC Metric to C, C++, and Java,"
*C++ Report*, June 1997.
[PDF](https://www.win.tue.nl/~wstomv/edu/2ip30/references/ABCmetric.pdf);
[Wikipedia](https://en.wikipedia.org/wiki/ABC_Software_Metric).

**What it measures.** *Size* (explicitly not complexity), as a triplet counting:

- **A** = Assignments (data stored/transferred into a variable)
- **B** = Branches (explicit forward branch out of scope, in practice function
  calls)
- **C** = Conditionals (boolean/logic tests: `if`, `else`, comparisons)

**Formula.** Vector `<A, B, C>`; scalar magnitude
`|ABC| = sqrt(A^2 + B^2 + C^2)`, rounded to nearest tenth.

**What it captures that others miss.** A layout-independent, cross-language "how
much does this code do" measure; useful for comparing size across languages. B
loosely mirrors CC but ABC deliberately ignores nesting/recursion.

**Weaknesses.** Often misused as complexity; counting rules are language-specific;
no validated defect thresholds.

## 8. Information Flow / Fan-in and Fan-out (Henry-Kafura)

**Origin.** Sallie Henry and Dennis Kafura, "Software Structure Metrics Based on
Information Flow," *IEEE TSE* SE-7(5):510 to 518, Sept 1981.
[Text mirror](https://masters.donntu.ru/2020/fknt/mazalov/library/article04/).

**What it measures.** Structural complexity from a procedure's connectivity to its
environment (information flow through parameters, globals, data structures).

**Definitions.**

- **fan-in** = number of local flows *into* the procedure + number of data
  structures it reads.
- **fan-out** = number of local flows *out* + number of data structures it
  updates.

**Formula.**

```text
complexity(SP) = length * (fan-in * fan-out)^2
```

`length` = LOC of the procedure. The product `fan-in * fan-out` represents all
input-to-output path combinations; squaring reflects the belief that complexity
is super-linear in connections (same power-of-2 rationale as Brooks' law and
Belady's partitioning formula). Some later sources drop `length` or drop the
square; the canonical 1981 form is `length * (fan-in * fan-out)^2`.

**What it captures that others miss.** *Inter-module* coupling / architectural
complexity, invisible to intra-function metrics like CC or Halstead.

**Weaknesses.** Zero fan-in or fan-out zeroes the whole metric; hard to compute
language-agnostically (needs call-graph and global/data-structure analysis); the
squaring is heuristic.

**Empirical evidence.** Henry and Kafura validated against UNIX maintenance-change
data with reported strong correlation, one of the few early metrics with real
maintenance-data validation.

## 9. Object-Oriented Metrics (Chidamber-Kemerer suite)

**Origin.** S. Chidamber and C. Kemerer, "A Metrics Suite for Object-Oriented
Design," *IEEE TSE* 20(6), 1994 (from the 1991 MIT Sloan "Towards a Metrics Suite
for OO Design").
[Suite PDF](https://people.scs.carleton.ca/~jeanpier/sharedF14/T1/extra%20stuff/about%20metrics/Chidamber%20&%20Kemerer%20object-oriented%20metrics%20suite.pdf);
[Virtual Machinery Sidebar 3](http://www.virtualmachinery.com/sidebar3.htm).

| Metric | Definition | Computation |
|---|---|---|
| **WMC** (Weighted Methods per Class) | Sum of complexities of a class's methods | Often simplified to method count; or sum of per-method CC |
| **DIT** (Depth of Inheritance Tree) | Longest inheritance path to the class root | Max ancestor chain length |
| **NOC** (Number of Children) | Immediate subclasses | Count direct subclasses |
| **CBO** (Coupling Between Objects) | Number of other classes this class is coupled to (calls/instantiates/uses) | Count distinct referenced classes |
| **RFC** (Response For a Class) | Methods potentially executed in response to a message | Local methods + distinct methods they call |
| **LCOM** (Lack of Cohesion of Methods) | Cohesion deficit among methods (see section 10) | Method-pairs not sharing attributes minus pairs that do |

**What they capture that others miss.** Design-level complexity (inheritance,
coupling, cohesion) that procedural metrics cannot express.

**Weaknesses / criticisms.** WMC conflates "many methods" with "complex methods";
DIT/NOC ambiguous (deep inheritance = reuse *or* fragility); measurement-theory
critiques argue CBO/LCOM lack a sound empirical relation system
([C&K measurement-theory perspective](https://www.researchgate.net/publication/3187794)).

**Empirical evidence.** Basili, Briand and Melo, "A Validation of Object-Oriented
Design Metrics as Quality Indicators," *IEEE TSE* 22(10), 1996: CBO, WMC, RFC,
DIT, NOC significantly predicted fault-proneness in student C++ projects; LCOM did
not. A NASA study found higher CBO/WMC meant lower quality. This is one of the
better-validated metric families for defect prediction (though confounded, again,
with size).

## 10. Cohesion, LCOM Variants (detailed)

Reference:
[NDepend, Lack of Cohesion of Methods](https://blog.ndepend.com/lack-of-cohesion-methods/);
[Aivosto cohesion metrics](https://www.aivosto.com/project/help/pm-oo-cohesion.html).

| Variant | Author(s) | Formula / definition |
|---|---|---|
| **LCOM1** | Chidamber and Kemerer 1991 | Number of method pairs sharing **no** attribute. (Zero for very different classes, a flaw.) |
| **LCOM2** | Chidamber and Kemerer 1994 | `max(0, P - Q)` where P = pairs not sharing attributes, Q = pairs that share at least one. |
| **LCOM3** | Li and Henry 1993 | Number of **connected components** in the graph where nodes = methods, edges = shared instance attribute. |
| **LCOM4** | Hitz and Montazeri | Connected components where edges = shared field **or** one method calls the other. LCOM4 = 1 is cohesive; 2 or more suggests splitting. (Recommended variant; accounts for accessors/calls.) |
| **LCOM HS** (LCOM5) | Henderson-Sellers | `(M - (sum mf)/F) / (M - 1)`, where M = methods, F = fields, sum mf = sum over fields of number of methods accessing that field. Range 0 to 2; 0 = perfect cohesion. |

**Signal:** LCOM4 (connected components) is the most interpretable, it literally
tells you into how many independent classes a class could be split.

## 11. Lines-of-Code Variants

Reference:
[Source lines of code (Wikipedia)](https://en.wikipedia.org/wiki/Source_lines_of_code);
[Beszedes et al., LOC definition differences in free tools](http://www.inf.u-szeged.hu/~beszedes/research/SED-TR2014-001-LOC.pdf).

- **Physical SLOC (PLOC/LOC):** Text lines in source; tools disagree on whether
  comments/blanks count. Easiest to compute, most style-sensitive.
- **Logical SLOC (LLOC/eLOC):** Programming statements (e.g., semicolon count in
  C-like languages; `for` + `printf` = 2 LLOC on one physical line). More
  style-independent but language-dependent.
- **CLOC** (comment lines), **BLOC** (blank lines), **DLOC** (documented lines),
  **ELOC** (executable lines).
- **Comment ratio / comment density:** `CLOC / (LOC)` or `CLOC / LLOC`, a
  readability proxy, also the disputed COM term in the original MI.

**Classic example:** `for (int i=0;i<100;i++) printf("hi"); /* greet */` gives 1
physical LOC, 2 LLOC, 1 comment line.

**Weaknesses.** No cross-tool standard (deviations 0.1 to 0.5% even among careful
tools); trivially gameable; but "a program with higher LLOC almost certainly does
more." **Crucial caveat:** LOC is the confounder against which every other metric
must be judged, many complexity metrics correlate strongly with it, so
incremental predictive value over LOC is the key question.
([getDX, LOC shortcomings](https://getdx.com/blog/lines-of-code/))

## 12. Indentation / Whitespace-Entropy Proxy (Hindle et al.)

**Origin.** Abram Hindle, Michael W. Godfrey, Richard C. Holt, "Reading Beside the
Lines: Indentation as a Proxy for Complexity Metric," ICPC 2008, pp. 133 to 142.
[PDF](https://plg.uwaterloo.ca/~migod/papers/2008/icpc08-abram.pdf); journal
extension
[Science of Computer Programming](https://www.sciencedirect.com/science/article/pii/S0167642309000379).

**What it measures.** Uses the **statistical moments of per-line indentation**
(mean, standard deviation, and higher moments like skew/kurtosis, plus entropy of
indentation levels) as a lightweight, language-independent, diff-friendly proxy
for classical complexity.

**Computation.** For a code fragment, extract the leading-whitespace count of each
line; compute summary statistics (mean, variance/SD, skewness, kurtosis) and/or
Shannon entropy over the distribution of indentation levels. No parser required,
works on multi-language diffs.

**What it captures that others miss.** Language-agnostic complexity ranking of
*revisions/diffs* where parsers fail or code mixes languages. The companion SCAM
2008 paper "From indentation shapes to code structures" maps indentation shapes
(flat / slash / bubble) to syntactic structure.

**Empirical evidence.** Indentation moments are **linearly and rank-correlated
with McCabe and Halstead metrics**, indentation performs "very similar to more
complex measurements such as McCabe." This is the key result underpinning any
language-agnostic tool: whitespace alone approximates structural complexity.
Posnett, Hindle and Devanbu later showed a 3-feature readability model (size,
complexity, **entropy** of tokens), entropy being a close cousin of this idea,
outperformed Buse-Weimer.

## 13. Readability / Cognitive-Load Metrics

### 13a. Buse-Weimer Readability

**Origin.** Raymond P.L. Buse and Westley R. Weimer, "Learning a Metric for Code
Readability," *IEEE TSE* 36(4):546 to 558, 2010.
[Preprint](https://web.eecs.umich.edu/~weimerw/p/weimer-tse2010-readability-preprint.pdf).

**What it measures.** A machine-learned binary classifier of "readable/not" from
**25 local syntactic features** (avg/max line length, avg number of identifiers,
avg use of brackets/parentheses/punctuation, indentation, comment counts, keyword
counts, etc.). Trained on 120 CS students rating 100 Java snippets 1 to 5.

**Results.** Around 80% accuracy (better than the average human at predicting the
human consensus), Pearson 0.63 with mean human scores. Correlates with three
software-quality signals: **code churn, automated defect reports, and
defect-related log messages**, the first readability metric empirically tied to
defects. PCA showed 6 of 25 features carry most variance; the most
negative-impact features are number of identifiers and line length.

### 13b. Posnett-Hindle-Devanbu (simplified)

Three features only, **size (LOC), complexity (Halstead), token entropy**,
outperformed Buse-Weimer on the same data, via forward feature selection.

### 13c. Scalabrino et al., Textual/Comprehensive Readability

**Origin.** Scalabrino, Linares-Vasquez, Poshyvanyk, Oliveto: ICPC 2016
"Improving Code Readability Models with Textual Features" and JSEP 2018 "A
Comprehensive Model for Code Readability."
[2018 PDF](https://sscalabrino.github.io/files/2018/JSEP2018AComprehensiveModel.pdf).

**What it measures.** Adds **lexical/textual** features (source-code-to-comment
consistency, identifier specificity, textual coherence, comment readability,
number of concepts) on top of structural features, arguing code is a form of
natural-language text. Outputs a 0 to 1 readability score; distributed as a CLI
tool.

**Results.** Textual features *complement* structural ones; the combined model
beats all prior state-of-the-art (validated on 600+ snippets rated by 5000+
people). Top textual features: comments readability, textual coherence, number of
concepts.

**Weaknesses (all readability models).** Trained on Java student snippets;
generalize poorly across languages/paradigms; recent human-centered reassessments
([arXiv:2401.14936](https://arxiv.org/pdf/2401.14936)) and reactive-programming
studies ([arXiv:2110.15246](https://arxiv.org/pdf/2110.15246)) find them fragile.
Textual features require identifier/comment NLP (not trivially
language-agnostic).

## 14. Cross-Cutting Empirical Findings (the LOC confounder)

The dominant critique across the literature: **most complexity metrics correlate
strongly with size (LOC), so their incremental predictive power is small.**

- Shepperd 1988: CC "may be no more than a proxy" for LOC.
- ELOC / Halstead Volume / CC mutually correlated at around 0.90; combined they
  explain only around 28% of defect variance; only Halstead Volume, **nesting
  depth**, and number of procedures show even around 0.3 defect correlation
  ([scialert](https://scialert.net/fulltext/?doi=jse.2013.114.120)).
- Landman et al.: at very large scale, CC-SLOC correlation is only *moderate*, and
  improves with aggregation + power transforms, so "redundancy" is overstated but
  real.
- OO metrics (Basili 1996) predict faults but are also size-confounded.
- Bottom line: any new metric should be evaluated for signal **beyond LOC**, and
  metrics like nesting depth, cognitive complexity, information-flow coupling, and
  readability entropy are the candidates that plausibly add orthogonal signal.

## 15. Ranked Shortlist

### (a) Most predictive / useful (evidence-weighted)

1. **Lines of code (LLOC/SLOC)**, the baseline every study confirms as a strong
   (if crude) defect/effort correlate; the yardstick others must beat.
2. **Nesting depth (max/avg)**, repeatedly one of the few signals with defect
   correlation *beyond* LOC; cheap; captures what CC misses.
3. **Cognitive Complexity**, the only structural metric explicitly validated
   (moderately) against human understandability; strong tooling ecosystem;
   rewards decomposition.
4. **Halstead Volume / Effort**, decomposed variants show fault-prediction value
   and the best correlation with measured cognitive load in recent work.
5. **Buse-Weimer / Posnett readability (size + complexity + entropy)**, the only
   metrics tied to defects *through* human readability; the 3-feature form is
   compact.
6. **CK OO suite (CBO, WMC, RFC)**, best-validated design-level fault predictors
   (Basili 1996), for OO codebases.
7. **Cyclomatic Complexity**, ubiquitous and a genuine testability bound, but
   largely redundant with LOC for defect prediction; keep for test-effort
   estimation, not as a maintainability oracle.
8. **Information-flow (Henry-Kafura)**, captures architectural coupling others
   miss, with real maintenance-data validation, but heavy to compute.

### (b) Most practical to compute language-agnostically (for a git-history / diff-based tool)

1. **Physical/logical LOC and comment ratio**, trivial, language-independent
   (LLOC needs light per-language statement rules).
2. **Indentation moments + indentation entropy (Hindle)**, purpose-built for
   language-agnostic, diff-friendly complexity ranking; proven to proxy
   McCabe/Halstead using *only whitespace*. **Top pick for a tool that must avoid
   per-language parsers.**
3. **Nesting depth via indentation/brace tracking**, approximable from
   indentation alone; strong orthogonal signal.
4. **Token entropy** (Posnett), needs only a generic lexer, not a full parser;
   language-agnostic-ish.
5. **ABC / decision-keyword counts**, a keyword-regex approximation of CC/ABC is
   feasible without a full parser, at some accuracy cost.
6. **Halstead**, needs per-language operator/operand classification; *moderately*
   portable but tokenizer-sensitive.
7. **Cognitive and Cyclomatic Complexity**, need a real AST/CFG per language
   (accurate parsers), so least practical for a strictly language-agnostic
   pipeline; feasible if you adopt tree-sitter grammars.
8. **CK OO / Information-flow**, require symbol resolution and call/inheritance
   graphs; least practical language-agnostically.

**Recommendation for a git-log, read-only, multi-language tool:** the sweet spot
is the Hindle indentation-entropy family + nesting depth + LOC/comment ratios +
token entropy, all computable from raw diff/file text without language-specific
parsers, all with published evidence of proxying heavier structural metrics, and
all naturally diff/revision-oriented (matching an evolutionary per-revision
analysis model). Reserve AST-based Cognitive/Cyclomatic Complexity for a future
tree-sitter-backed tier if per-language accuracy becomes a requirement.

## Primary Sources

- McCabe 1976: [A Complexity Measure (PDF)](http://literateprogramming.com/mccabe.pdf)
  and [Cyclomatic complexity (Wikipedia)](https://en.wikipedia.org/wiki/Cyclomatic_complexity)
- Shepperd 1988 critique:
  [A critique of cyclomatic complexity (PDF)](https://www.cs.du.edu/~snarayan/sada/teaching/COMP3705/lecture/p1/cycl-1.pdf)
- SonarSource Cognitive Complexity:
  [Whitepaper PDF](https://www.sonarsource.com/docs/CognitiveComplexity.pdf),
  [Blog](https://www.sonarsource.com/blog/cognitive-complexity-because-testability-understandability/),
  [clang-tidy impl](https://clang.llvm.org/extra/clang-tidy/checks/readability/function-cognitive-complexity.html)
- Munoz Baron et al. 2020: [arXiv:2007.12520](https://arxiv.org/pdf/2007.12520);
  Lavazza critique:
  [JSS 2022](https://www.sciencedirect.com/science/article/abs/pii/S0164121222002370)
- Halstead:
  [Wikipedia](https://en.wikipedia.org/wiki/Halstead_complexity_measures),
  [Verifysoft](https://www.verifysoft.com/en_halstead_metrics.html),
  [Design analysis](https://www.researchgate.net/publication/260843757_An_Analysis_of_the_Design_and_Definitions_of_Halstead's_Metrics),
  [Decomposed Halstead fault prediction](https://peerj.com/articles/cs-1647/)
- Maintainability Index:
  [NDepend](https://blog.ndepend.com/maintainability-index/),
  [van Deursen critique](https://avandeursen.com/2014/08/29/think-twice-before-using-the-maintainability-index/),
  [Microsoft Learn](https://learn.microsoft.com/en-us/visualstudio/code-quality/code-metrics-maintainability-index-range-and-meaning?view=visualstudio),
  [MI variants (MDPI)](https://www.mdpi.com/2076-3417/13/5/2972)
- NPATH: [Nejmeh 1988 (ACM)](https://dl.acm.org/doi/10.1145/42372.42379),
  [ACPATH (arXiv:1610.07914)](https://arxiv.org/pdf/1610.07914)
- Nesting:
  [Measuring nesting (IET 2022)](https://ietresearch.onlinelibrary.wiley.com/doi/full/10.1049/sfw2.12069),
  [Nesting problem](https://www.researchgate.net/publication/261455470_Cyclomatic_complexity_The_nesting_problem)
- ABC metric:
  [Fitzpatrick 1997 (PDF)](https://www.win.tue.nl/~wstomv/edu/2ip30/references/ABCmetric.pdf),
  [Wikipedia](https://en.wikipedia.org/wiki/ABC_Software_Metric)
- Henry-Kafura 1981:
  [text mirror](https://masters.donntu.ru/2020/fknt/mazalov/library/article04/)
- Chidamber-Kemerer 1994:
  [Suite PDF](https://people.scs.carleton.ca/~jeanpier/sharedF14/T1/extra%20stuff/about%20metrics/Chidamber%20&%20Kemerer%20object-oriented%20metrics%20suite.pdf),
  [Virtual Machinery](http://www.virtualmachinery.com/sidebar3.htm)
- LCOM variants:
  [NDepend](https://blog.ndepend.com/lack-of-cohesion-methods/),
  [Aivosto](https://www.aivosto.com/project/help/pm-oo-cohesion.html)
- SLOC/LLOC:
  [Wikipedia](https://en.wikipedia.org/wiki/Source_lines_of_code),
  [Beszedes LOC tool study (PDF)](http://www.inf.u-szeged.hu/~beszedes/research/SED-TR2014-001-LOC.pdf)
- Hindle et al. 2008:
  [ICPC PDF](https://plg.uwaterloo.ca/~migod/papers/2008/icpc08-abram.pdf),
  [journal version](https://www.sciencedirect.com/science/article/pii/S0167642309000379)
- Buse-Weimer 2010:
  [Preprint PDF](https://web.eecs.umich.edu/~weimerw/p/weimer-tse2010-readability-preprint.pdf)
- Scalabrino et al. 2018:
  [JSEP PDF](https://sscalabrino.github.io/files/2018/JSEP2018AComprehensiveModel.pdf)
- Defect-rate correlations:
  [scialert controlled experiment](https://scialert.net/fulltext/?doi=jse.2013.114.120),
  [Landman CC/SLOC (ResearchGate)](https://www.researchgate.net/publication/220204439_Cyclomatic_Complexity_and_Lines_of_Code_Empirical_Evidence_of_a_Stable_Linear_Relationship),
  [defect patterns (arXiv:1912.04014)](https://arxiv.org/pdf/1912.04014)
