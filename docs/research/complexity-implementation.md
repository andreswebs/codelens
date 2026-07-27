# Implementing Language-Agnostic Complexity Analysis in a Go CLI

Scope: how to actually build source-code complexity metrics (cyclomatic,
cognitive, and cheaper proxies) into a read-only Go tool. Three families are
covered: **parser-based** (tree-sitter et al.), **token/keyword heuristics**
(lizard, scc), and **indentation** (parser-free). Ends with a recommendation
matrix and a concrete Go integration path over git history.

## 1. Parser-based approaches

### 1.1 tree-sitter: how it works

tree-sitter is a parser generator plus an incremental parsing runtime (C
library). You feed it a grammar (`grammar.js`), it generates a GLR-based parser,
and at runtime it produces a **Concrete Syntax Tree (CST)** where every node maps
directly to a grammar symbol (including punctuation). Lexing happens on-demand
during parsing, not as a separate pass. Two defining features: **incremental
reparse** (only edited regions re-parse, milliseconds) and **error recovery**
(usable tree even for broken code).
([how it works](https://tree-sitter.github.io/tree-sitter/creating-parsers/3-writing-the-grammar.html),
[symflower overview](https://symflower.com/en/company/blog/2023/parsing-code-with-tree-sitter/),
[tomassetti incremental parsing](https://tomassetti.me/incremental-parsing-using-tree-sitter/))

**Getting grammars for many languages.** Each language is a separate project (a
generated C parser plus optional query files). There are 100+ grammars (JS,
Python, Go, Rust, C/C++, Java, TS, etc.). They are not bundled with the runtime,
you pull each grammar in. This is the central operational cost: you ship and
version N C parsers.
([tree-sitter topic](https://github.com/topics/tree-sitter))

**Go bindings, three options:**

| Binding                                                                     | Maintainer               | Grammars                                                                                                                               | Notes                                                                                                                                                                                                                                                                                                                                                             |
| --------------------------------------------------------------------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter)         | community                | many **bundled in-repo** (JS, Python, C/C++, Go, TS/TSX, Java, Ruby, etc.)                                                             | Long-established, MIT, `sitter.NewParser()` + `parser.ParseCtx(...)`. Distributed as a git snapshot rather than tagged releases. GC calls `Close()` for you. Easiest onboarding: one `go get`, grammars included. ([README](https://github.com/smacker/go-tree-sitter/blob/master/README.md), [pkg.go.dev](https://pkg.go.dev/github.com/smacker/go-tree-sitter)) |
| [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) | official tree-sitter org | **none bundled**, each grammar is a separate `go get` (e.g. `tree-sitter/tree-sitter-javascript`), or load `.so` at runtime via purego | Newer, tracks upstream. API: `NewParser()`, `SetLanguage()`, `Parse(code, nil)`, `tree.RootNode()`, `root.ToSexp()`. **You must call `Close()`** on Parser/Tree/TreeCursor/Query/QueryCursor (CGO finalizer bug). More assembly required but cleaner long-term.                                                                                                   |

Both require **CGO** (the runtime and every grammar are C). That is the single
biggest architectural consequence for a Go tool, see 1.5.

### 1.2 Computing cyclomatic / cognitive complexity by walking the CST

Once you have a CST, complexity is a tree walk:

- **Cyclomatic (McCabe):** start at 1, +1 for every decision node
  (`if_statement`, `for_statement`, `while_statement`, each `case`,
  `catch_clause`, `&&`, `||`, ternary). This is essentially what
  `go/ast`-based [gocyclo](https://github.com/fzipp/gocyclo) does natively for
  Go.
- **Cognitive (SonarSource):** three rules, (1) ignore shorthand structures; (2)
  +1 for each break in linear flow (loops, conditionals, catch, switch, sequences
  of `&&`/`||`, recursion); (3) an **extra +nesting-depth penalty** for nested
  structures. A `switch` counts once (not per-case), unlike cyclomatic. The
  reference implementation for Go is
  [uudashr/gocognit](https://github.com/uudashr/gocognit), which ports G. Ann
  Campbell's
  [Cognitive Complexity whitepaper](https://www.sonarsource.com/resources/cognitive-complexity/)
  directly onto `go/ast`.
  ([whitepaper](https://www.sonarsource.com/resources/cognitive-complexity/),
  [gocognit README](https://github.com/uudashr/gocognit/blob/master/README.md))

**The generalization problem, is there a query that counts branches across all
grammars?** Short answer: **no universal query.** tree-sitter queries are
S-expressions ("regex for syntax trees") but they are **bound to a specific
grammar**, a query only runs on nodes parsed with the language it was compiled
against, and node type names differ per grammar (`if_statement` in C,
`if_expression` in Rust, `if` in some grammars). The
[`tags.scm`](https://deepwiki.com/tree-sitter/tree-sitter-java/4.2-query-system)
convention exists only for code-navigation (defs/refs), and `highlights.scm`
captures keywords for coloring, neither is a portable "decision-point"
abstraction.
([query syntax](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/1-syntax.html),
[query system](https://deepwiki.com/tree-sitter/tree-sitter-java/4.2-query-system))

Practical consequence: to count complexity polyglot with tree-sitter you maintain
a **per-language table of node kinds** to count (one small `.scm` query or one Go
slice of node-type strings per grammar). That is the same maintenance shape as
scc's `languages.json`, you do not escape per-language config, you just get an
accurate tree.

### 1.3 ast-grep and semgrep for polyglot construct counting

- **[ast-grep](https://ast-grep.github.io/)** is a polyglot structural
  search/rewrite tool backed by tree-sitter (it matches on the CST). It has no
  built-in "count" primitive; you write a pattern or a YAML `rule` (with
  `kind`/`has`/`inside` relational operators) and count matches (via `--json`
  output or the Node API). Patterns are still language-specific. Usable to count
  constructs, but it is an external Rust binary, not a Go library.
  ([pattern deep dive](https://ast-grep.github.io/advanced/pattern-parse.html),
  [core concepts](http://astgrep.com/advanced/core-concepts.html))
- **[semgrep](https://github.com/semgrep/ocaml-tree-sitter-languages)** uses OCaml
  tree-sitter bindings and per-language grammars extended for its pattern syntax.
  Heavier, rule-oriented, OCaml core, not a fit to embed in a Go tool.

Both confirm the same lesson: tree-sitter gives accurate trees, but "count branch
constructs" is always expressed per-grammar.

### 1.4 How difftastic/semgrep use tree-sitter (evidence it scales)

[difftastic](https://deepwiki.com/Wilfred/difftastic/2.1-tree-sitter-integration)
parses with tree-sitter then lowers each grammar's CST into its own simplified
`Syntax` tree, with a per-language `TreeSitterConfig` (and `sub_languages` for
embedded langs like JS-in-HTML). Adding a language = one config entry + detection
rule. This is the canonical model for "normalize N grammars into one internal
representation," exactly what a complexity walker would do.
([adding a parser](https://difftastic.wilfred.me.uk/adding_a_parser.html))

### 1.5 Language-native AST vs universal parsers

|              | Native (`go/ast`, etc.)                                  | tree-sitter (universal)                   |
| ------------ | -------------------------------------------------------- | ----------------------------------------- |
| Accuracy     | Highest (semantic, type-aware)                           | High (syntactic CST; no types)            |
| Languages    | One per parser; only Go is free in-process for a Go tool | 100+ grammars                             |
| Dependencies | stdlib for Go; would need a toolchain per other lang     | CGO + N C grammars                        |
| Effort       | Trivial for Go, impractical to cover 20 langs            | Moderate: bindings + per-lang node tables |
| Robustness   | Fails on non-compiling/partial files                     | Error-recovers                            |

For a **polyglot** tool, native parsers do not scale (you would embed 20
compilers). tree-sitter is the only realistic accurate-parser path, at the cost
of CGO.

## 2. Heuristic / parser-free approaches

### 2.1 lizard (terryyin/lizard), tokenize, do not parse

lizard deliberately avoids full parsing ("Header-Free CCA"): it computes how
complex code _looks_, not what it _is_, so it needs no headers/imports/build
context. Pipeline: a **regex tokenizer** (`generate_tokens`) emits tokens;
processor stages strip comments/whitespace, count NLOC and tokens, detect function
boundaries via a **state machine / nesting stack**, and a `condition_counter`
increments CCN whenever a token is in the reader's `conditions` set.
([lizard.py](https://github.com/terryyin/lizard/blob/master/lizard.py),
[repo](https://github.com/terryyin/lizard))

**Exact default decision set** (from the base `CodeReader`): `if`, `for`,
`while`, `catch`, `case`, `&&`, `||`, `?`. Language readers extend it (e.g.
Python adds `elif`/`assert`). CCN starts at 1 per function; each hit = +1. It
also tracks `max_nesting_depth` and offers `-ENS` (count nested control
structures) and `-m` (modified CCN: whole switch = 1).
([code_reader.py](https://raw.githubusercontent.com/terryyin/lizard/master/lizard_languages/code_reader.py),
[clike.py](https://raw.githubusercontent.com/terryyin/lizard/master/lizard_languages/clike.py))

- **Coverage:** around 20 languages (C/C++, Java, C#, JS/TS, ObjC, Swift, Python,
  Ruby, PHP, Scala, Go, Rust, Kotlin, Lua, Solidity, Fortran, Erlang, GDScript,
  TTCN-3).
- **Accuracy:** Good per-function CCN approximation; gives **per-function**
  granularity (its real advantage over scc) because it detects function
  boundaries. Fails on macro/DSL-heavy code and unusual syntax.
- **Go effort:** No Go port exists, you would reimplement the tokenizer + state
  machine per language, or shell out to Python. Reimplementing function-boundary
  detection for 20 languages is the expensive part.

### 2.2 scc (boyter/scc) and tokei, LOC, and scc's "complexity"

Both are pure-Go/Rust LOC counters using a **byte-level state machine** (aware of
strings vs comments vs code). scc is itself **pure Go**, so its internals are
directly reusable/vendorable.
([scc repo](https://github.com/boyter/scc))

**scc's "complexity"** is _not_ cyclomatic: while scanning code state, if it sees
a branch keyword/operator from that language's list in
[`languages.json`](https://github.com/boyter/scc), e.g. Java's
`for if switch while else || && != ==`, it does +1. It is a **whole-file sum**,
"almost free" CPU-wise, with two documented caveats: only comparable **within the
same language** (do not compare across langs unweighted), and it **misses
recursion** (no AST). Good for ranking files: `scc --by-file -s complexity`.
([Discussion #235](https://github.com/boyter/scc/discussions/235),
[README](https://github.com/boyter/scc/blob/master/README.md))

**scc's COCOMO:** classic Boehm effort model. Three profiles (a,b,c,d): organic
`2.4,1.05,2.5,0.38`, semi-detached `3.0,1.12,2.5,0.35`, embedded
`3.6,1.20,2.5,0.32`. Effort = a*KLOC^b*EAF; schedule = c*Effort^d; cost =
effort*avg-wage\*overhead. Tunable via `--eaf`, `--avg-wage` (default around
$56k), `--overhead` (2.4), `--cocomo-project-type`. Coverage: 200+ languages.
tokei is similar for LOC but has no complexity metric.

### 2.3 The "count decision keywords" heuristic, accuracy and failure modes

Counting `if/for/while/case/catch/&&/||/?` (the lizard/scc approach):

- **Approximation quality:** correlates well with true cyclomatic in mainstream
  imperative code; scc/lizard exist precisely because it is "good enough."
- **Failure modes:** (a) keyword-in-string/comment false positives (state machine
  mitigates); (b) misses **recursion** and dynamic dispatch; (c) `switch`/pattern-
  match counted as N vs 1 depending on config; (d) language keyword drift
  (`elif`, `when`, `select`, guard clauses) needs per-language lists; (e)
  macros/generated code inflate counts. Whole-file counting (scc) loses
  per-function signal; token-stream + boundary detection (lizard) recovers it.

### 2.4 Indentation / whitespace complexity, fully parser-free

Adam Tornhill's proxy (from _Software Design X-Rays_ /
[indent-complexity-proxy](https://github.com/adamtornhill/indent-complexity-proxy)):
strip blank lines and comments, normalize tabs to spaces, then count **logical
indentation** per line (leading whitespace / spaces-per-indent). Configurable
`-s/--spaces` (default 4) and `-t/--tabs` (default 1). Aggregate via sum / mean /
stdev / max. The premise: indentation strongly correlates with branching/nesting
depth across virtually all languages.
([indent-complexity-proxy](https://github.com/adamtornhill/indent-complexity-proxy),
[feststelltaste walkthrough](https://www.feststelltaste.de/calculating-indentation-based-complexity/),
[CodeScene complexity trends](https://docs.enterprise.codescene.io/versions/4.4.24/guides/technical/complexity-trends.html))

- **Coverage:** truly language-neutral (same code works for Java, JS, C++,
  Clojure, YAML, and so on). Academic backing: Hindle/Godfrey/Holt,
  ["Reading beside the lines"](https://plg.uwaterloo.ca/~migod/papers/2008/scam08.pdf),
  indentation statistics correlate with McCabe/Halstead; raw vs logical
  indentation barely differs.
- **Accuracy vs true AST:** lowest of the three, but the _cheapest possible_ and
  best as a **trend** signal. Tornhill's key move: pair it with LOC over git
  history, flat LOC + rising indentation = complexity growth. CodeScene ships this
  as "complexity trends."
- **Failure modes:** sensitive to reindentation/style changes (need to know the
  date of a style flip), Python-style significant whitespace inflates it, and it
  treats some genuinely complex flat constructs as simple. Tabs-vs-spaces
  normalization is mandatory.
- **Go effort:** trivial, a few dozen lines, no dependencies, no CGO. This is the
  natural first metric to add.

## 3. Go ecosystem

- **Native Go complexity (Go source only):**
  [fzipp/gocyclo](https://github.com/fzipp/gocyclo) (cyclomatic) and
  [uudashr/gocognit](https://github.com/uudashr/gocognit) (cognitive, ports the
  SonarSource whitepaper), both walk `go/ast`, accurate, per-function, zero CGO.
  Great if you ever want a high-accuracy Go-only lane. Forks:
  [nrnrk/gocognito](https://github.com/nrnrk/gocognito),
  [SVilgelm/gocognit](https://github.com/SVilgelm/gocognit).
- **Go tree-sitter bindings:**
  [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) (grammars
  bundled, easiest) and official
  [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter)
  (grammars separate, purego runtime-loading option). Both need **CGO** (unless
  you use the purego `.so`-loading path, which trades CGO for shipping shared
  libraries).
- **Go token/LOC counters:** scc is **pure Go** and vendorable, its state-machine
  scanner and `languages.json` are a ready-made foundation for a keyword-counting
  complexity lane with 200+ languages "for free."
- **No Go port of lizard** exists; its per-language function-boundary logic would
  have to be reimplemented or shelled out.

**What it realistically takes to add complexity to codelens:**

- _Indentation lane:_ around 1 day, no deps. Read blob bytes, normalize
  whitespace, sum logical indents. Instant polyglot coverage.
- _Keyword lane:_ days-to-weeks. Either vendor scc's scanner + keyword tables
  (whole-file, 200+ langs) or reimplement lizard-style tokenizing for per-function
  CCN on the around-10 langs you care about.
- _tree-sitter lane:_ weeks + ongoing. CGO build, ship N grammars, maintain
  per-grammar decision-node tables, cross-compilation/binary-size pain. Buys
  accurate cyclomatic + cognitive per-function.

## 4. Computing over git history

codelens is currently stdin-only and never runs git. Adding complexity means
reading **file content at revisions**, which is the new capability. Options:

- **Shell out to one long-lived `git cat-file --batch --buffer`** and feed
  `<rev>:<path>` lines on stdin, draining each blob reader fully before the next
  request (the protocol is strictly sequential). `--buffer` is the key throughput
  flag; use `--batch-check` when you only need type/size. This is the
  Gitea/Gitaly-proven pattern.
  ([git-cat-file docs](https://git-scm.com/docs/git-cat-file),
  [Gitea #6649](https://github.com/go-gitea/gitea/issues/6649),
  [Gitaly catfile](https://pkg.go.dev/gitlab.com/gitlab-org/gitaly/internal/git/catfile)).
  Note: this breaks codelens's current "never runs git" invariant; you would
  consume blob content on stdin instead, or add an explicit opt-in.
- **Pure-Go via [go-git](https://github.com/go-git/go-git):** reads
  packfiles/loose objects directly, no subprocess, no `git` binary dependency, a
  cleaner fit for a read-only tool, at the cost of go-git's own performance
  profile on huge repos.

**Scaling tactics** (complexity at many revisions is O(revisions x files)):

- **Blob-hash caching:** a file's blob SHA is identical across commits when
  unchanged; key the complexity cache by blob SHA so each unique blob is parsed
  once, not once per commit. This is the single biggest win, most files are
  unchanged in most commits.
- **Only recompute changed paths** per commit (diff-driven), carrying forward
  cached values.
- **Sample revisions** (e.g. monthly) for trend lines rather than every commit.
- Cheaper metrics (indentation, keyword) make full-history sweeps feasible;
  tree-sitter parsing at every revision is where cost explodes, reserve it for
  HEAD or sampled snapshots.

## 5. Recommendation matrix

| Dimension                         | Indentation proxy      | Keyword/token (scc/lizard style)           | tree-sitter (parser)                     | Native `go/ast` (gocyclo/gocognit) |
| --------------------------------- | ---------------------- | ------------------------------------------ | ---------------------------------------- | ---------------------------------- |
| Language coverage                 | Any (universal)        | around 20 (lizard) / 200+ (scc) via tables | 100+ grammars                            | Go only                            |
| Accuracy vs true AST metric       | Low (trend proxy)      | Medium (good CCN approx; misses recursion) | High (syntactic; cognitive+cyclomatic)   | Highest                            |
| Granularity                       | Line/file              | File (scc) or function (lizard)            | Function                                 | Function                           |
| Go impl effort                    | Trivial (around 1 day) | Medium (vendor scc / reimpl lizard)        | High (CGO + per-grammar tables)          | Trivial (existing libs)            |
| Performance                       | Fastest                | Very fast                                  | Slower (parse cost)                      | Fast                               |
| Dependencies                      | None                   | None (pure Go if scc-based)                | **CGO + N C grammars** (or purego + .so) | stdlib                             |
| Robustness on partial/dirty files | High                   | High                                       | High (error recovery)                    | Low (needs compilable)             |

### Recommendation for codelens (polyglot, read-only, Go)

Adopt a **layered, opt-in "lane" model**, the same two-lane philosophy already in
the dataviz skill notes:

1. **Ship indentation-based complexity first.** Zero dependencies, no CGO,
   universal, and paired with LOC over git history it delivers the highest-value
   signal (complexity _trend_/growth) that matches codelens's evolutionary-
   analysis identity. This is the 80/20.
2. **Add a keyword-counting lane by vendoring scc's pure-Go scanner +
   `languages.json`.** Gives a defensible "cyclomatic-ish" per-file number across
   200+ languages with no CGO and no new build complexity. Label it honestly as an
   approximation, comparable only within a language.
3. **Treat tree-sitter as an optional high-accuracy lane, gated behind a build
   tag.** Only if users demand true per-function cyclomatic/cognitive across
   languages. It forces CGO, grammar shipping, per-grammar decision-node tables,
   and heavier binaries, a real tax on a currently-clean Go tool. If you go this
   route, use
   [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) for fastest
   onboarding (bundled grammars) or the official binding's purego path to avoid
   CGO at the cost of shipping `.so` files.
4. **Keep a pure-Go native lane in mind for Go-heavy repos:** wire
   [gocyclo](https://github.com/fzipp/gocyclo)/[gocognit](https://github.com/uudashr/gocognit)
   for exact metrics on `.go` files with no CGO.

For git-history computation, cache by **blob SHA** regardless of lane, and prefer
go-git (pure Go) to preserve a clean no-subprocess story, falling back to
`git cat-file --batch --buffer` only if go-git performance is inadequate on large
repos.

The through-line: there is **no free universal accurate metric**. Every accurate
path (tree-sitter or native) needs per-language knowledge anyway; the cheap
universal paths (indentation, keyword-counting) are proxies. For a polyglot
evolutionary-analysis tool, the proxies plus trend analysis are the
highest-leverage starting point, with parser-based accuracy as an optional
upgrade.

## Key sources

- tree-sitter:
  [how grammars work](https://tree-sitter.github.io/tree-sitter/creating-parsers/3-writing-the-grammar.html),
  [query syntax](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/1-syntax.html),
  [smacker binding](https://github.com/smacker/go-tree-sitter),
  [official Go binding](https://github.com/tree-sitter/go-tree-sitter),
  [difftastic integration](https://deepwiki.com/Wilfred/difftastic/2.1-tree-sitter-integration),
  [ast-grep](https://ast-grep.github.io/)
- Heuristics: [lizard](https://github.com/terryyin/lizard)
  ([base conditions](https://raw.githubusercontent.com/terryyin/lizard/master/lizard_languages/code_reader.py)),
  [scc](https://github.com/boyter/scc)
  ([complexity discussion](https://github.com/boyter/scc/discussions/235)),
  [indent-complexity-proxy](https://github.com/adamtornhill/indent-complexity-proxy),
  ["Reading beside the lines" paper](https://plg.uwaterloo.ca/~migod/papers/2008/scam08.pdf)
- Cognitive complexity:
  [SonarSource whitepaper](https://www.sonarsource.com/resources/cognitive-complexity/),
  [gocognit](https://github.com/uudashr/gocognit),
  [gocyclo](https://github.com/fzipp/gocyclo)
- Git history: [git-cat-file](https://git-scm.com/docs/git-cat-file),
  [Gitea batch indexing](https://github.com/go-gitea/gitea/issues/6649),
  [Gitaly catfile](https://pkg.go.dev/gitlab.com/gitlab-org/gitaly/internal/git/catfile)
