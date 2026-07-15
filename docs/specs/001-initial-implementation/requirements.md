# codelens (Initial Implementation) - Requirements

## Introduction

codelens is a command-line tool that mines the history of a git repository and
runs evolutionary analyses over it: which files change together, which are
hotspots, who owns what, how churn accumulates, and how old the code is. It is a
faithful reimplementation of the analyses in code-maat, repackaged with a
predictable, machine-readable interface intended to be driven by both humans and
autonomous agents.

Users produce a git log with a documented command, pipe it into codelens, and
choose one analysis per invocation. codelens emits a structured result by
default, so the output can be consumed programmatically without scraping, and it
can also render human-friendly tables or spreadsheet-ready CSV on request. Every
command is self-describing: an agent can ask codelens what a command accepts and
what columns it returns, without external documentation.

This document specifies the observable behavior of the first complete version.
It covers input ingestion, the 20 analyses, output formats and shaping, the
optional aggregation transforms, introspection, error reporting, and the
supporting commands. It describes what the tool does and why, not how it is
built.

## Requirements

### 1. Log ingestion

**User Story**: As a user, I want to feed a git history log into codelens with
minimal friction, so that I can analyze a repository without memorizing an exact
export format.

**Acceptance Criteria**:

- WHEN no input source is specified, the system shall read the log from standard
  input.
- WHEN a `--log FILE` option is provided, the system shall read the log from
  that file.
- WHEN `--log -` is provided, the system shall read the log from standard input.
- The system shall accept the documented git log format (a per-commit header of
  short-hash, date, and author, optionally followed by the commit subject, plus
  per-file added/deleted/path lines).
- WHERE a commit's file line reports a binary change (a dash for added/deleted),
  the system shall treat the added and deleted counts as zero.
- WHERE an entry contains multiple stacked commit headers (as some merges or
  pull requests produce), the system shall attribute the entry to the last
  header.
- WHEN a `--input-encoding` option is provided, the system shall decode the log
  using that encoding instead of the UTF-8 default.
- IF the log cannot be parsed, THEN the system shall report an input error that
  names the offending entry and line and points the user to the log-generation
  helper.
- IF the log is empty, THEN the system shall report an empty-input error.

### 2. Analyses

**User Story**: As a user, I want to run any of the established code-evolution
analyses, so that I can investigate design and organizational risks in my
codebase.

**Acceptance Criteria**:

- The system shall provide each analysis as its own named command.
- The system shall support the following analyses: authors, revisions, coupling,
  sum-of-coupling, summary, absolute-churn, author-churn, entity-churn,
  entity-ownership, main-developer, refactoring-main-developer, entity-effort,
  main-developer-by-revisions, fragmentation, communication, messages, code-age,
  and a parse (raw parsed-record) output.
- The system shall expose each analysis under a descriptive canonical command
  name and shall also accept the reference tool's terse name as an alias (for
  example, main-developer aliased by main-dev, absolute-churn by abs-churn,
  sum-of-coupling by soc).
- The system shall produce, for each analysis, the same result columns and the
  same row ordering as the reference tool for equivalent input.
- WHERE the parse (raw parsed-record) output is requested, the system shall emit
  the records in log order, as parsed.
- The system shall sort each analysis's rows deterministically so that repeated
  runs on identical input produce identical ordering.
- WHEN an analysis that requires line-change counts is run against a log lacking
  them, the system shall report an input error explaining that modification
  metrics are missing.
- WHERE the coupling analysis is run, the system shall accept thresholds for
  minimum revisions, minimum shared revisions, minimum coupling percentage,
  maximum coupling percentage, and maximum change-set size, and shall exclude
  results outside those thresholds.
- WHERE the coupling analysis is run with a verbose option, the system shall
  include the per-pair revision-count detail columns in addition to the standard
  columns.
- WHERE the code-age analysis is run, the system shall compute each entity's age
  in whole calendar months relative to a "time now" date, interpreting dates as
  UTC and defaulting to the current UTC date when no such date is provided.
- WHERE the messages analysis is run, the system shall require a matching
  expression and count, per entity, the commits whose message matches it.
- IF the messages analysis is requested but no matching expression is provided,
  THEN the system shall report a usage error.
- IF the messages analysis is run against a log that carries no commit messages,
  THEN the system shall report an input error explaining that messages are
  unavailable.

### 3. Output format

**User Story**: As an agent or script, I want structured output by default, so
that I can consume results without parsing prose or guessing columns.

**Acceptance Criteria**:

- The system shall emit results as a structured JSON result by default, in every
  context, whether or not the output is a terminal.
- The JSON result shall include a schema version, a success indicator, the
  analysis name, the effective parameters, the row count, and the result rows.
- WHEN an analysis produces no rows, the system shall emit a successful result
  with an empty row set and a zero row count.
- WHEN `--format ndjson` is requested, the system shall emit one result row per
  line without an enclosing wrapper, uniformly for every analysis (including
  those whose rows are statistic/value pairs, such as summary).
- WHEN `--format csv` is requested, the system shall emit comma-separated rows
  with a header, matching the reference tool's column names and ordering on a
  best-effort basis.
- WHEN `--format table` is requested, the system shall emit an aligned,
  human-readable table.
- The system shall write only results to standard output, keeping diagnostics
  off that stream.

### 4. Output shaping

**User Story**: As an agent, I want to limit the size and fields of results, so
that I protect my limited context window.

**Acceptance Criteria**:

- WHEN a `--fields` option lists field paths, the system shall emit only those
  fields, always retaining the schema version and success indicator.
- IF a requested field path is not valid for the analysis, THEN the system shall
  report a usage error that lists the valid field paths.
- WHEN a `--rows N` option is provided, the system shall emit at most N rows,
  applied after sorting.
- WHEN `--rows` truncates the result, the system shall report the pre-truncation
  total row count and indicate that the result was truncated.
- The system shall apply `--rows` to every output format.
- The `--fields` option shall apply to the structured JSON output.
- WHERE a non-JSON format is selected, the system shall ignore `--fields`.

### 5. Architectural grouping

**User Story**: As a user, I want to aggregate files into logical components, so
that I can analyze the system at an architectural level rather than per file.

**Acceptance Criteria**:

- WHEN a `--group` option references a mapping definition, the system shall remap
  each file to the first matching logical group before analysis.
- The system shall parse the grouping definition as pattern-to-name text lines
  by default, and as a structured (JSON) list when a grouping-format option
  selects JSON.
- The system shall interpret an unanchored pattern as a path-prefix match and an
  anchored pattern as a full expression.
- WHERE a file matches no group, the system shall exclude it from the analysis.
- IF the grouping definition is malformed, THEN the system shall report an input
  error.

### 6. Temporal aggregation

**User Story**: As a user, I want to treat commits within a rolling time window
as a single logical change, so that coupling analyses are not skewed by many
small commits.

**Acceptance Criteria**:

- WHEN a `--temporal-period N` option is provided, the system shall combine
  commits within a sliding window of N days into single logical change sets
  before analysis.
- WHILE aggregating a window, the system shall count each file at most once per
  window.
- IF the temporal period is not a positive integer, THEN the system shall report
  a usage error.
- The system shall document that temporal aggregation is intended for coupling
  analyses.

### 7. Team mapping

**User Story**: As a user, I want to map individual authors to teams, so that I
can compute organizational metrics at the team level.

**Acceptance Criteria**:

- WHEN a `--team-map` option references an author-to-team mapping, the system
  shall substitute each author with their mapped team before analysis.
- The system shall parse the mapping as author,team CSV by default, and as a
  structured (JSON) form when a team-map-format option selects JSON.
- WHERE an author is absent from the mapping, the system shall keep that author
  unchanged.
- IF the mapping is malformed, THEN the system shall report an input error.

### 8. Introspection

**User Story**: As an agent, I want to discover what a command accepts and
returns at runtime, so that I can use it without pre-supplied documentation.

**Acceptance Criteria**:

- The system shall provide a schema command that lists every available command
  with a short description.
- WHEN the schema command is given a specific command, the system shall report
  that command's options (name, type, default, whether required), its output
  columns with per-column descriptions, its possible error codes, and its
  possible exit codes, as structured data.
- The system shall keep the reported schema consistent with the command's actual
  behavior.

### 9. Log-command helper

**User Story**: As a user, I want codelens to tell me exactly how to generate a
compatible log, so that I never have to guess the export command.

**Acceptance Criteria**:

- The system shall provide a command that prints the exact git log command
  needed to produce a compatible log.
- The printed command shall include the commit subject so that all analyses,
  including messages, are supported.
- WHERE a start date is supplied to the helper, the system shall include a
  corresponding date-window argument in the printed command.

### 10. Errors and exit status

**User Story**: As a script or agent, I want errors reported predictably and
distinguishable by category, so that I can branch on outcomes reliably.

**Acceptance Criteria**:

- The system shall report all errors on the standard error stream as structured
  data containing a stable code, a message, and, where useful, a hint and
  details.
- WHEN an invocation succeeds, the system shall exit with status 0, including
  when the result set is empty.
- IF an invocation has a usage error (unknown command or option, missing or
  invalid option value), THEN the system shall exit with status 2.
- IF an invocation has an input error (unparseable, empty, or unsuitable log, or
  a malformed grouping or team mapping), THEN the system shall exit with status 3.
- IF an invocation fails unexpectedly, THEN the system shall exit with status 1.
- WHILE a debug option is enabled, the system shall include diagnostic detail
  (such as stack traces) on the error stream.
- WHILE a debug option is not enabled, the system shall not expose internal
  stack traces to the user.

### 11. Help and versioning

**User Story**: As a user, I want standard help and version information, so that
I can orient myself and report the exact build I am running.

**Acceptance Criteria**:

- WHEN invoked with no command, the system shall print usage help and exit
  successfully.
- WHEN a help option is requested for any command, the system shall print that
  command's usage, options, and a short description.
- WHEN version information is requested, the system shall print the build
  version.

### 12. Agent knowledge packaging

**User Story**: As an agent, I want ready-to-consume guidance shipped with the
tool, so that I can operate it correctly from the start of a conversation.

**Acceptance Criteria**:

- The system shall ship agent-facing guidance describing the piped-input
  workflow, the log-command helper, the runtime schema discovery, output-shaping
  options, and the exit-code taxonomy.
- The guidance shall state the invariants an agent should follow (bounding output
  with field and row limits, discovering a command via the schema command).

### 13. Result fidelity

**User Story**: As a user migrating from the reference tool, I want equivalent
numeric results, so that I can trust codelens as a drop-in analytical
replacement.

**Acceptance Criteria**:

- The system shall reproduce the reference tool's computed values for equivalent
  input, including its rounding behavior for percentages, averages, and
  ownership ratios.
- The system shall be validated against the reference tool's test corpus.

### 14. Input safety

**User Story**: As an operator running codelens on untrusted logs or
definitions, I want malformed or hostile input rejected quickly and safely, so
that the tool fails fast instead of hanging or misbehaving.

**Acceptance Criteria**:

- WHEN a user-supplied regular expression (a grouping pattern or a message
  expression) is provided, the system shall reject it with a usage error if it
  is invalid or exceeds a complexity/size bound, rather than attempting an
  unbounded match.
- IF log or definition content contains disallowed control characters (such as
  NUL), THEN the system shall report an input error.
- The system shall open all input files read-only and shall write results only
  to standard output.
  </content>
