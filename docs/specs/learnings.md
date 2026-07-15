# Learnings

Running log of non-obvious problems solved and decisions made during
implementation, for the next person.

## P0-2 internal/terr (cod-jw0u)

- The design (cli-design.md §7, ticket) lists `New(code, exit, hint, msg)` with
  no wrapped-error parameter, yet specifies `Error()` should append
  `": <wrapped>"` and `Unwrap()` should return the cause. The "New with
  wrapped" phrasing in the TDD cases is satisfied by a `Wrap(err error) *Error`
  method (copy-returning, receiver unchanged), mirroring `WithDetails`. This
  keeps package-level sentinels immutable and reusable while allowing
  per-callsite wrapping.
- The canonical wrapping idiom is still `fmt.Errorf("%w: ctx", sentinel)`:
  because `%w` wraps the `*Error` pointer, `errors.As(err, &coded)` and
  `errors.Is(err, sentinel)` both work without any custom `Is`/`As`. `Wrap` is
  for attaching a non-coded cause to a coded error.
- `WithDetails`/`Wrap` return a shallow copy (`c := *e`). Fine here since fields
  are value/interface types; revisit if `*Error` ever holds a pointer/slice
  that should not be shared.

## P0-3 internal/output (cod-g7yh)

- errcheck vs. the house "no `_ =`" rule: the rule bans silencing errors you
  _ought to handle_, but `.golangci.yml` explicitly sanctions the acknowledged
  blank-assignment idiom (like a deferred `Close`). `EmitError` writes to the
  diagnostic sink (stderr) where a write failure is unrecoverable, so it renders
  the whole message to one string and does a single `_, _ = io.WriteString(w, s)`.
  Internal JSON marshal errors are _not_ discarded: they fall back to the text
  rendering. Keeping the write best-effort let `EmitError` stay a void function
  (matching the ticket signature) without a returned error the callers would
  only ignore at the top-level exit boundary anyway.
- urfave/cli v3 has no exported usage-error type to type-assert, so
  `classifyUsageError` matches message substrings: `flag provided but not
defined`, `no such flag`, `not set` (required-flag errors read `Required flag
"x" not set`), and `invalid value` (bad flag/arg value). Verified against the
  v3.10.1 source (command.go, command_parse.go, args.go, errors_test.go). This
  is the exit-2 boundary; downstream cod-ic8y refines classification.
- Envelope omitempty: `total_count`/`truncated` are omitted when zero/false so
  an uncapped result is unambiguous; `--rows` truncation (later ticket) sets
  both. `rows` has no omitempty and serializes `[]` for an empty-but-valid run.

## P0-1 remove greet; wire root app (cod-vdx3)

- Testability: `main()` is `os.Exit(run(os.Args, os.Stdout, os.Stderr))` and all
  behavior lives in `run(args []string, stdout, stderr io.Writer) int`, so tests
  drive it with `bytes.Buffer`s and assert the exit code directly.
- urfave/cli v3 calls `os.Exit` from its default `ExitErrHandler`, which crashes
  a test process. Set `ExitErrHandler` to a no-op so `Run` returns the error and
  `run()` owns exit-code mapping (`output.ExitCodeFor`) and rendering
  (`output.EmitError`).
- An unrecognized command is NOT surfaced as a classifiable usage error. v3
  routes it to the help command as a topic, returning `No help topic for 'X'`,
  which `classifyUsageError` does not match (would map to exit 1). Fix: set the
  `CommandNotFound` hook (`func(ctx, cmd, name)`, no error return) to capture the
  name; it fires even with zero subcommands, suppresses the help-topic routing,
  and makes `Run` return nil. `run()` then synthesizes a coded
  `terr.New("usage_error", 2, hint, msg)` carrying `WithDetails` `{command: name}`.
- `--debug` gates verbose diagnostics only: on error under `--debug`, a
  `slog.JSONHandler` bound to the injected `stderr` writer logs the error in
  addition to the coded envelope; without `--debug`, only the one-line coded
  envelope is emitted. Binding slog to the passed writer (not the global default)
  keeps the trace testable.
- `--format` is read via a bound `Destination` string (default `json`); if flag
  parsing fails before it is set, it stays `json`, so `EmitError` always has a
  valid format.

## P1-2 gitlog tokenizer (cod-y20g)

- New `internal/gitlog` package; its package doc comment lives in `tokenize.go`.
- `tokenize(io.Reader) iter.Seq2[[]string, error]` streams line-by-line. The
  error is delivered as a trailing `(nil, err)` yield (standard Go 1.23+ range-
  over-func convention); no partial entry is emitted alongside an error. P1-3's
  `Parse` should treat any yielded error as terminal.
- Silent truncation guard: `bufio.Scanner`'s default 64 KiB token cap would
  truncate a huge line without error, so the buffer is raised to
  `maxLineSize` (1 MiB) via `sc.Buffer`; a line past that surfaces as
  `bufio.ErrTooLong` through the iterator instead of corrupting the entry.
- Blank detection uses `strings.TrimSpace(line) == ""` so whitespace-only lines
  delimit too; content lines are kept verbatim (only the terminator is stripped,
  which also drops a trailing CR, giving free CRLF handling).
- Encoding boundary: tokenize works on already-decoded text. `--input-encoding`
  decoding belongs in P1-3 `Parse` (wrap the reader before it reaches tokenize),
  per design §5/reference §3.5. Do not add it here.

## P1-3 gitlog parser (cod-3ksh)

- Prelude vs. numstat discrimination is by the literal `--` prefix, and it is
  unambiguous: a binary numstat line is `-\t-\t<path>`, whose first two bytes are
  `-` then TAB, so it never matches `HasPrefix(line, "--")`. numstat count fields
  are always digits or a single `-`, so a numstat line can't be mistaken for a
  prelude. `parseEntry` therefore just consumes leading `--` lines as preludes
  and treats the rest as numstat.
- Subject parsing exploits `strings.Split(line, "--")`: the leading `--` makes
  `fields[0]` empty, so `fields[1..3]` are hash/date/author and `fields[4:]`
  rejoined with `--` reconstructs a subject that itself contains `--` (design
  §3.2, ticket case "refactor: split a--b module"). Missing subject (stock
  3-field log) -> `fields` len 4 -> message `-`.
- Stacked preludes: overwrite `prelude` on each leading `--` line so the LAST
  wins (merge/PR parity, reference §3.4). Don't accumulate.
- Scope boundary: errors are plain `fmt.Errorf` with a `git log entry N:` prefix.
  Named `terr` codes (parse_error/empty_log) and NUL/control-char rejection are
  deliberately deferred to cod-rf77; ported fixture goldens to cod-ggqz. Kept
  `Parse(io.Reader, model.Options)` signature so rf77 can layer coded errors and
  encoding decoding without changing the surface.

## P2-1 analysis descriptor + registry (cod-joym)

- The analysis registry is a process-global: a `descriptors` name->Descriptor
  map plus a `byKey` alias index, `sync.RWMutex`-guarded, that analyses
  `Register` from `init`. Downstream tickets registering into it need isolation:
  in-package tests use an unexported `resetRegistry()` helper that reassigns the
  two maps under the lock. Reuse that pattern rather than depending on
  registration order across tests.
- `Register` validates all keys (name + aliases) against the existing index AND
  against a per-call `seen` set before mutating, so a name/alias duplicated
  within one descriptor panics too, and no partial state is left on panic.
- `Descriptor.Run` is `func([]model.Modification, Opts) (output.Result, error)`.
  By design, cross-cutting steps are kept OUT of Run: group/temporal/team-map
  transforms run in the pipeline BEFORE Run; `--rows` truncation and `--fields`
  projection run in the output layer AFTER Run. `Opts` is a superset (each
  analysis reads only its fields) and deliberately omits group/temporal/teammap
  because those are already applied to the modification set by the time Run sees
  it.

## P4-0 analysis/calc grouping + rounding helpers (cod-s8uc)

- New package `internal/analysis/calc` centralizes the parity-critical rounding
  so every Phase-4 analysis shares one pinned implementation. Reference is
  research §7.
- `CentiRatio(own, total)` reproduces `ratio->centi-float-precision` = two
  SIGNIFICANT digits, not two decimals. Implemented as
  `ParseFloat(FormatFloat(r, 'g', 2, 64))`: `'g'` precision is significant
  digits, so 0.834->0.83 and 0.0834->0.083 both hold. `total < 1` is clamped to
  1 (matches original's max(total,1) guard) so 5/0 -> 5.0.
- `TruncInt` uses `math.Trunc` (toward zero) to match the original `int(...)`;
  `Ceil` is `math.Ceil`. Coupling degree = `TruncInt(Percentage(shared/avg))`,
  average-revs = `Ceil(Average(a,b))`.
- `GroupBy[T]` returns `[]Group[T]` sorted ASCENDING by key (documented
  divergence from code-maat's dataset insertion order) so downstream sort and
  `--rows` truncation are deterministic; items within a group keep first-seen
  order. `Distinct[T comparable]` preserves first-seen order. Prefer these over
  ad-hoc map iteration in analyses to avoid nondeterministic output.

## P2-2 analysis authors, the default (cod-sskw)

- First real analysis; establishes the per-analysis file pattern for Phase 4.
  Layout: one file `authors.go` with the row struct (`authorsRow` with
  `snake_case` json tags), a `runAuthors(mods, Opts) (output.Result, error)`,
  and an `authorsDescriptor() Descriptor` constructor called from `init()` via
  `Register`.
- Descriptor is exposed as a FUNCTION, not a package var. In-package tests
  (`registry_test.go`) call `resetRegistry()` which wipes the global, so a test
  must never assert on init-registered state; instead call `authorsDescriptor()`
  directly to inspect Name/Aliases/RowSchema/Run. Reuse this pattern for the
  other analyses so their tests stay independent of registration order.
- `Run` builds the FULL envelope itself: `SchemaVersion: output.SchemaVersion`,
  `OK: true`, `Analysis`, `RowCount`, `Rows`. `EmitJSON` does not populate any
  fields. Initialize rows as `make([]T, 0, n)` so an empty result marshals to
  `[]`, not `null`.
- authors: NAuthors = `len(calc.Distinct(authors-in-group))`, NRevs =
  `len(group.Items)` (row count per entity, i.e. original revisions-in=nrows,
  NOT distinct revs). Sort desc by [n_authors, n_revs] then entity ASC as the
  final tiebreak (documented divergence for deterministic --rows). `GroupBy`
  already returns entity-asc groups, so a `sort.SliceStable` with the explicit
  entity tiebreak is stable and self-documenting.
- empty_log is enforced at the parse layer (cod-rf77), so `runAuthors(nil, ...)`
  returns an OK empty result rather than an error.

## P1-4 gitlog named parse errors + control chars (cod-rf77)

- `errors.Is(err, ErrParse)` does NOT work when the error was built with
  `ErrParse.WithDetails(...).Wrap(...)`: both return shallow COPIES of the
  sentinel (by design, so sentinels stay reusable), so pointer identity is lost
  and `*terr.Error` has no custom `Is`. Assert coded errors by `Code()` instead
  (via `errors.As(err, &terr.Coded)`), which is also more robust. The
  alternative `fmt.Errorf("%w", ErrParse)` keeps identity but then you can't
  attach details. Downstream error assertions should compare codes, not identity.
- Empty-log vs. empty-result distinction lives in `Parse`: it counts tokenized
  entries and returns `ErrEmptyLog` only when `entryNum == 0` (empty/whitespace-
  only input). A well-formed entry with a prelude but no numstat (empty merge)
  still counts as an entry, so such a log parses to `[]` with no error. The old
  `TestParse_Empty` (`"" -> []`) was reconciled to `TestParse_EmptyLog`.
- The offending source line is surfaced by an unexported `entryError{line, err}`
  returned from `parseEntry` at each failure site (missing prelude -> `lines[0]`;
  bad prelude/date -> the prelude; bad numstat -> that numstat line). `Parse`
  pulls it out with `errors.As` and attaches `{"entry":N,"line":L}` via
  `WithDetails` plus a human `entry N, line %q` message. Details is a
  `map[string]any` (JSON-marshaled straight into the envelope's `details`).
- Control-char rejection is in the TOKENIZER (`tokenize.go`), per scanned line,
  before blank-line/entry logic, so it fires anywhere in the input and streams.
  `hasControlChar` allows `\t` (numstat separators) and rejects bytes `< 0x20`
  or `== 0x7f`; the scanner already stripped `\n`/trailing `\r`, and bytes
  `>= 0x80` (UTF-8 sequences) are allowed. `ErrControlChar` reuses code
  `parse_error` (exit 3), same as `ErrParse`, so don't distinguish them by code.

## P2-3 cmd: command tree + input wiring (cod-f0zl)

- Global flags (`--log`, `--format`, `--rows`, `--input-encoding`, `--group*`,
  `--team-map*`, `--temporal-period`, `--debug`) are registered on the ROOT
  command, not on each subcommand. urfave/cli v3 inherits non-`Local` flags into
  every subcommand's flag set, and `Command.lookupFlag` walks `Lineage()`, so a
  subcommand `Action` reads them with `cmd.String/Int/Bool` and they parse when
  placed AFTER the subcommand name (`codelens authors --log f --rows 1`). No
  need to duplicate the global flag set per command or share flag pointers
  (which would alias `Destination` state).
- `run()` gained a `stdin io.Reader` param (signature now
  `run(args, stdin, stdout, stderr)`); `main()` passes `os.Stdin` and tests pass
  `strings.NewReader(...)`. The root's `Reader` is set to stdin and the per-command
  `Action` closes over the same reader for `openLog`. Existing `main_test.go`
  calls were updated to thread the extra arg.
- `--rows` truncation is generic over `Result.Rows` (typed as `any`, holding a
  concrete `[]someRow`): `truncate()` uses `reflect.Value.Slice(0,n)` after
  guarding `Kind()==Slice`, then sets `RowCount=n`, `TotalCount=total`,
  `Truncated=true`. `n<=0` means "all". Truncation happens after the analysis's
  own sort and before emit, so it is stable and format-independent.
- Per-analysis `Opts` are populated only for flags the descriptor actually
  declares (`analysisOpts` builds a `declared` set from `d.Flags`). `cmd.Int` on
  an undefined flag returns 0 safely, but gating by declaration keeps a command
  from silently adopting another analysis's threshold and documents intent.
- Scope boundaries deferred to their own tickets (don't re-solve here): real
  `--format` bodies + enum validation -> P2-4 (cod-9eay); a `--format` Validator
  triggers urfave's help-on-error dump to STDOUT (the anti-pattern the design
  forbids), and suppressing that is P5-4 (cod-ic8y); applying the
  group/temporal/team-map transforms -> P3-1 (cod-0xx4). Until P2-4, `emitResult`
  emits the JSON envelope for every `--format` value.

## P5-1 print-log-command (cod-xx0y)

- `print-log-command` is a plain top-level `*cli.Command`, not an analysis
  registry entry: it emits a copy-paste shell command (stdout, no JSON
  envelope), so it does not fit the descriptor/Run/emit pipeline. Wired via
  `main.go` as `Commands: append(analysisCommands(stdin), printLogCommand())`.
- Validating `--after` _inside the Action_ (returning a coded `terr` usage
  error) keeps stdout clean: an error returned from an Action goes straight to
  the no-op `ExitErrHandler` and back to `run()`, so urfave does NOT print its
  help-on-error dump. That dump only happens for urfave-level flag-parse errors
  (the concern deferred to cod-ic8y); a well-typed string flag whose _value_ we
  reject ourselves never triggers it. Confirmed by the empty-stdout assertion in
  TestPrintLogCommand_BadAfter.
- Date strictness: `time.Parse("2006-01-02", s)` alone is lenient about zero
  padding (accepts `2024-1-1`). A round-trip check
  `t.Format(layout) != s` rejects unpadded input so `--after` means exactly
  YYYY-MM-DD; genuinely malformed/out-of-range dates fail `Parse` first.

## P0-4 output/fields projection (cod-vqjh)

- `Result.Rows` is typed `any` (it holds a concrete `[]someRow`), so validation
  cannot enumerate nested paths from the `Result` type alone. `ValidateFields`
  reflects over the envelope _value_, dereferencing pointers and interfaces to
  reach the dynamic type. Callers that want `rows.<col>` validated must pass a
  Result whose Rows is populated (or at least a typed empty slice).
- Empty-slice trick: when reflecting a slice/array of len 0, use
  `reflect.New(v.Type().Elem()).Elem()` to synthesize a zero element and still
  collect its nested json-tag paths. So an analysis that returns zero rows keeps
  a discoverable/validatable `rows.<col>` path set.
- Projection operates on the _decoded JSON tree_ (`json.Unmarshal` into `any`),
  not on Go types: `ProjectFields([]byte, []string)` builds a nested "projection
  tree" from the dotted paths (nil leaf = keep subtree whole, map = descend) and
  copies only selected keys. This decouples projection from the concrete row
  type and guarantees byte-shape parity with `EmitJSON` (both go through
  `json.Marshal`). `schema_version` and `ok` are force-added to the tree so the
  output is always a recognizable envelope.
- `"*"` wildcard segment matches all map keys, handled in both directions:
  `collectValidPaths` emits a `<prefix>.*` path for map fields (plus present
  keys), and `applyProjection` expands `"*"` to every key of the object. Path
  matching falls back to a segment-wise compare so a requested `params.foo`
  matches a valid `params.*`.
- golangci-lint (govet `inline`) rejects the deprecated `reflect.Ptr` alias and
  demands `reflect.Pointer`. Use `reflect.Pointer` in new code.

## P2-4 output formats: json/ndjson/csv/table (cod-9eay)

- Import-cycle constraint drove the `Emit` signature. The ticket specified
  `Emit(..., schema []analysis.Column, ...)`, but `analysis` imports `output`
  (Descriptor.Run returns `output.Result`), so `output` cannot import
  `analysis`. `Emit` instead takes `columns []string` (the ordered snake_case
  row-schema Names); the cmd layer maps `d.RowSchema` via a small
  `columnNames(d)` helper. Only names+order are needed for csv/table, so no
  information is lost.
- Numeric cells use `json.Decoder.UseNumber()`. Decoding rows to
  `map[string]any` the naive way turns every JSON number into `float64`, and
  `fmt`-ing a large integer float yields scientific notation (`1e+06`).
  Decoding as `json.Number` keeps the exact source text for both ints and the
  centi-float degrees, so csv/table cells match json byte-for-byte.
- ndjson preserves row field order by splitting the marshaled rows array into
  `[]json.RawMessage` and writing each raw object, rather than re-marshaling a
  `map` (which would sort keys). csv/table go through the map form because they
  look up values by column key, where order comes from the schema not the map.
- One row-extraction seam: `rowObjects(rows any) ([]json.RawMessage, error)`
  marshals `res.Rows` once and splits it; `rowMaps` builds on it for csv/table.
  This keeps json struct tags the single source of truth for keys across all
  formats and treats a nil/`"null"` rows value as zero rows (clean empty
  result) instead of an error.
- Unknown `--format` is a coded `usage_error` (exit 2) with the offending value
  in details, not a silent JSON fallback (the prior placeholder `emitResult`
  ignored format entirely). `--fields` is honored only on the json branch;
  csv/table/ndjson ignore it by construction. `--rows` truncation stays in the
  cmd Action (before Emit), so every format formats exactly the surviving rows.

## P2-5 cmd: schema introspection (cod-x9ol)

- The schema _builder_ (`analysis.Schema(Descriptor)` and
  `analysis.List(analyses, extra)`) lives in `internal/analysis/schema.go` and
  is built purely from Descriptors, so the per-command schema (flags, row_schema,
  codes) cannot drift from behavior. `List` takes the descriptor slice as a
  PARAMETER rather than calling `All()` internally, so it stays testable without
  touching the process-global registry (which `resetRegistry` wipes).
- `analysis.Flag`/`analysis.Column` were untagged metadata structs; added
  `json:"..."` tags so they marshal to snake_case (`name/type/default/required/desc`,
  `name/type/desc`). Safe: they were not marshaled anywhere before.
- Meta commands (schema, print-log-command, version) have no analysis
  Descriptor, so their command-list summaries are declared in the cmd layer
  (`metaSummaries()` in schema.go), next to where the commands are wired. Their
  one-line usages are extracted to consts (`printLogCommandUsage`, `schemaUsage`,
  `versionUsage`) and reused for both the `cli.Command.Usage` and the list entry
  so the two can't drift. `version` is listed even though its subcommand lands in
  P5-2 (cod-7fb5): the list advertises the documented surface (design §4), and
  the const is the natural home P5-2 will reuse.
- `schema --command CMD` resolves through `analysis.Lookup`, so aliases resolve
  to the same canonical schema for free. Unknown CMD is a coded `usage_error`
  (exit 2) carrying `details.known_commands` (sorted canonical analysis names) so
  an agent recovers without a `schema` round trip. `--command` describes
  analyses only; meta commands appear in the list but are not `--command`
  targets (they have no Descriptor to reflect).
- Slice fields in the schema/list envelopes are normalized to non-nil so
  `aliases`/`flags`/etc. marshal as `[]` not `null`; an agent can iterate them
  unconditionally. Reuse the `nonNil*` helpers when adding schema fields.

## P1-5 gitlog ported fixtures + golden tests (cod-ggqz)

- `.local/refs/code-maat` is a DANGLING symlink in the working env (points at
  `/Users/andre/...`), so the code-maat corpus (`git2_test.clj`,
  `simple_git2.txt`) could not be copied verbatim. Fixtures under
  `src/internal/gitlog/testdata/*.log` are reconstructed faithful to the
  documented git2(+subject) format (reference doc §3.2-3.4) and the existing
  inline `parse_test.go` constants (which are themselves derived from code-maat).
  GPL-3.0 origin/derivation is documented in `testdata/README.md`. If the real
  corpus becomes available, regenerate with `-update` and diff.
- Attribution lives in `testdata/README.md`, NOT inside the `.log` files: the
  git2 log format has no comment syntax, so any header line would be a parse
  error. README is the idiomatic Go testdata home for provenance.
- Goldens are `json.MarshalIndent([]model.Modification)` → PascalCase keys,
  because `model.Modification` deliberately carries no JSON tags (output-layer
  concern). This is fine for an internal test artifact; the golden pins parser
  output, not the public envelope shape.
- Standard golden pattern: package-level `var update = flag.Bool("update", ...)`
  in the test file; `-update` writes goldens and returns early, else read+compare
  bytes. `TestGoldens_Reviewed` is a separate drift guard asserting
  `entries.log` == exactly 6 records, so an unreviewed `-update` that changes the
  record count fails independently of the byte comparison.
- `simple_git2.log` intentionally repeats entities (git.clj, git2.clj across two
  commits each) and authors so the downstream authors/coupling e2e (cod-i10e and
  the P4 batch) has non-trivial signal to assert on.

## P2-6 e2e authors golden tests (cod-i10e)

- The spine-freezing e2e lives in `cmd/codelens` (package `main`) and drives the
  real `run(args, stdin, stdout, stderr) int` entry point, not the analysis in
  isolation: it exercises flag parsing, the registry-built command tree, the
  pipeline, and `output.Emit` end-to-end. One table (`authorsCases`) covers
  json/ndjson/csv/table + `--fields rows.entity` + `--rows 2` + `schema
--command authors`, each asserting exit 0 and empty stderr before the byte
  golden compare.
- Input fixture `cmd/codelens/testdata/authors.log` is the ported code-maat
  `simple_git2.txt` content (4 entities, a 2-author entity, a tie broken by
  entity name, and a binary numstat). Reusing it means the authors result is
  hand-verifiable and matches the parser fixtures. GPL-3.0 provenance is in
  `testdata/README.md` (attribution never goes in `.log` files: git2 has no
  comment syntax).
- `--fields` reorders top-level keys to `ok,rows,schema_version`: projection
  rebuilds the envelope from a `map[string]any`, and `encoding/json` sorts map
  keys. Deterministic, so it goldens cleanly; just don't expect struct field
  order to survive projection.
- The `-update` flag is package-scoped (`var update = flag.Bool(...)`), same
  pattern as gitlog's golden test - no collision because each package compiles
  its own test binary. `TestE2E_Authors_JSONReviewed` is the drift guard: it
  decodes the JSON envelope and asserts 4 rows with git2.clj ranked first, so a
  blind `-update` that broke the sort or count fails even if all goldens were
  rewritten together.
- Global flags (`--format`, `--fields`, `--rows`) are placed before the
  subcommand name in argv; urfave inherits root flags either side, but the
  before-subcommand position is what the design documents as canonical.
- These goldens are now the frozen output contract for the whole surface; the
  20 Phase-4 analyses are additive and should mirror this test's shape.

## P4-7 churn core helpers (cod-migh)

- First "core helper" subpackage (`internal/analysis/churn`) with only helpers
  and no registered analysis yet. Because every helper except
  `ErrMissingMetrics` is unexported, `unused` (staticcheck U1000) would flag
  them as dead code, but same-package `_test.go` usage counts: `churn_test.go`
  exercises each one, so lint stays green. Consequence for the six downstream
  churn analyses: they must live in this same package to reach the unexported
  helpers (or the helpers get exported when wired). Noted on the ticket.
- `requireLoc` deliberately treats an empty `mods` slice as OK (nil), not a
  metrics error. Absence of data is `empty_log` handled upstream; the guard
  only fires when there IS data and none of it carries loc (a message-only
  log). git2 always has numstat, so in practice this only bites hand-crafted or
  3-field logs.
- Binary rows need no special-casing in the sums: the parser already normalizes
  `-`/`-` numstat to `LocAdded=0, LocDeleted=0` (with `Binary=true`), so
  straight summation yields the code-maat "binary counts 0" behavior while the
  row still counts toward distinct-commit totals.
- Used a named `entityContribs` type instead of the anonymous struct in the
  ticket's signature sketch - idiomatic and keeps struct literals readable in
  tests; unexported types don't trip revive's exported-doc rule.

## P4-4 coupling core algorithms (cod-1sde)

- Second core-helper subpackage (`internal/analysis/couplingalgo`), same
  unexported-helpers + same-package-tests pattern as churn. Core funcs
  (`changeSetsByRevision`, `coChangingByRevision`, `moduleByRevs`,
  `couplingFrequencies`) and `pair` are unexported; `unused` stays green because
  `couplingalgo_test.go` exercises them.
- Import-cycle constraint made explicit: this package is imported by the
  `analysis` package, so it must NOT import `analysis.Opts` back. `WithinThreshold`
  therefore takes a small local `couplingalgo.Opts` (the four coupling thresholds).
  The consuming coupling/soc analyses populate it from `analysis.Opts`. This is
  the general rule for any `analysis/<helper>` subpackage that needs tuning
  options.
- Self-pairs are the load-bearing subtlety and are handled in two opposite ways
  on purpose: `coChangingByRevision` retains `[A,A]` (via selections-with-
  replacement then sort+distinct) so `moduleByRevs` can count a module changed
  alone in a revision; `couplingFrequencies` drops `[A,A]` so shared-rev counts
  cover only genuine cross-entity coupling. A regression that unifies these two
  would silently break both per-module totals and coupling degrees.
- `pair` is a canonical sorted struct `{A, B}` (A <= B) rather than a slice, so
  it is `comparable` and usable directly as a `map[pair]int` key for frequency
  counts.

## P4-14 effort core helper (cod-7s7l)

- Third core-helper subpackage (`internal/analysis/effort`), same pattern as
  churn/couplingalgo. Ports `effort.clj` sum-effort-by-author: `TotalRevs` is the
  entity ROW count (the original's `nrows`) and per-author `Revs` is that author's
  ROW count within the entity (Clojure `frequencies`). Both count ROWS, not
  distinct revisions -- a file listed twice in one change set counts twice.
  Pinned by `TestEffort_CountsRowsNotDistinctRevs`.
- Export-visibility rule for the effort batch: unlike churn/couplingalgo whose
  helpers stay unexported, the effort helper is EXPORTED (`ByEntity`,
  `EntityEffort`, `AuthorRevs`). The batch-D analyses (entity-effort,
  main-dev-by-revs, fragmentation, communication) live in package `analysis`
  (e.g. `analysis/entityeffort.go`), not inside package `effort`, and the ticket
  designs reference the helper cross-package as `effort.ByEntity`. Ticket sketches
  spelled it lowercase `byEntity`; that is shorthand -- a cross-package call
  cannot reach an unexported symbol, so export it.
- `TotalRevs` is duplicated onto every `AuthorRevs` row (rather than sitting once
  on `EntityEffort`) so consumers can compute an author's share
  (`Revs/TotalRevs`) straight from a flattened row without carrying the entity
  total separately. Matches the original's output-row shape (`:author-revs`,
  `:total-revs` on each row).

## P4-5 coupling analysis, incl. --verbose (cod-uwv3)

- The batch-B analyses (coupling, soc) live in package `analysis`
  (`analysis/coupling.go`), not inside `couplingalgo`, so they cannot reach that
  package's unexported helpers. Rather than export the four internal helpers
  (`changeSetsByRevision`, `coChangingByRevision`, `moduleByRevs`,
  `couplingFrequencies`) piecemeal, added ONE exported assembly function
  `couplingalgo.Couplings(mods, maxChangesetSize) []PairRevs` that returns each
  real pair's shared count and both entities' own revision totals, sorted by
  (entity, coupled). Keeps the load-bearing self-pair subtleties encapsulated;
  soc (cod-rbbk) will need its own exported entry (change sets per rev, NO max
  filter) since Couplings applies the size drop and is coupling-specific.
- `WithinThreshold`'s `revs int` arg: code-maat passes the RAW average-revs (a
  ratio) to `within-threshold?`, but our closed `WithinThreshold` takes an int.
  Pass `calc.TruncInt(avg)` (= floor for non-negative revs). This is EXACTLY
  faithful because for an integer bound n, `floor(x) >= n` iff `x >= n`, so the
  inclusive `>= min-revs` check is unchanged. Do NOT pass the emitted
  `ceil(avg)` here: ceil would flip the boundary (e.g. avg 4.5, min-revs 5 ->
  ceil 5 wrongly passes). The emitted `average_revs` column is still `Ceil(avg)`.
- degree = `TruncInt(Percentage(shared/avg))` (truncate toward zero), so 76.92 ->
  76; max-coupling bound is `floor(degree) <= max` (in WithinThreshold, applied
  to the float percentage, not the truncated int). A fixture with A in 8 revs, B
  in 5, shared 5 gives avg 6.5 -> degree 76, average_revs 7, pinning both trunc
  and ceil in one case.
- --verbose columns are `*int` with `omitempty` (first/second/shared_revisions);
  nil in the standard result so json emits exactly the 4 documented columns and
  csv/table (which key off RowSchema column names) never show them. RowSchema
  lists all 7 columns always, with the verbose three marked "(--verbose only)"
  in their desc -- matches the schema-conformance expectation that RowSchema is
  static per descriptor.
- No `Params` set on the Result: follows the authors precedent (closed,
  golden-tested). There is still no cross-analysis params convention; if one
  lands later it should apply uniformly rather than being bolted onto coupling.
- No original .clj fixtures are present in the repo despite the ticket citing
  `logical_coupling_test.clj`; tests use hand-computed values derived directly
  from the reference-doc formulas (§6 Coupling, §7 rounding).

## cod-rbbk sum-of-coupling (soc)

- soc did NOT need a `couplingalgo` helper after all (the prior note anticipated
  one). soc is a plain per-revision accumulation: for each rev's distinct-entity
  set of size k, every member gains k-1, summed across all revs. It needs no
  pair frequencies, no self-pair bookkeeping, and no changeset-size drop. So
  `runSoc` builds change sets inline with `calc.GroupBy`(by rev) + `calc.Distinct`
  and sums into a `map[string]int`. Reaching into couplingalgo would have coupled
  soc to coupling-specific machinery (the max-changeset filter especially) for no
  benefit. Keep shared code shared only where the subtlety is genuinely shared.
- Filter is STRICT `soc > min-revs`, unlike coupling's inclusive `>=`. Verified
  with a size-6 set (members soc 5, excluded at min-revs 5) vs a size-7 set
  (soc 6, kept).
- Sort is `[soc, entity]` BOTH descending (reference doc §6). This differs from
  authors/coupling where the entity tie-break is ascending; do not assume a
  uniform tie-break direction across analyses -- follow the per-analysis spec.

## cod-kyfe transform/group (layer mapping)

- Error taxonomy split by _source of the data_, not by kind of failure: a bad
  `--group` definition (missing `=>`, empty pattern/name, oversize, uncompilable
  regex, malformed JSON, unknown format) is a USAGE error (`invalid_group`,
  exit 2) because the definition arrives via a flag. Contrast gitlog, where a
  bad log is an INPUT error (exit 3) because the log is the analyzed data. When
  adding a transform, classify its parse failures by where the bytes came from.
- Anchoring (ports code-maat grouper): a `^`-prefixed pattern compiles verbatim;
  any other pattern becomes `^<pattern>/` (path-prefix, trailing slash required).
  So `src/Features/Core` matches `src/Features/Core/x.cs` but NOT the bare path
  `src/Features/Core` nor `src/Features/CoreX/...`. First matching spec wins, so
  order specs most-specific-first. Unmatched entities are dropped.
- Go regexp is RE2 (linear, no catastrophic backtracking), so a length cap
  (1000 chars) is the only guard needed against a pathological pattern; there is
  no backtracking-blowup class of input to defend against.
- `.local/refs` was empty again (same as the coupling tickets), so the three
  `*-layers-definition.txt` fixtures are hand-authored to the documented
  `pattern => name` syntax with GPL attribution in `testdata/README.md`, not
  ported byte-for-byte. Text format has no comment syntax (a `#` line has no
  `=>` and would error), so attribution lives in the README, not inline.

## cod-en35 transform/temporal (sliding N-day grouping)

- Sliding window uses Clojure `(partition n 1 days)` semantics: fixed size n,
  step 1, and the incomplete trailing window is DROPPED. Consequence: when the
  padded day range [first,last] is shorter than the period, there is no complete
  window and the result is empty. Implemented as `for start := 0; start+period
<= len(days); start++`. Tested explicitly (TestApply_PeriodExceedsSpan).
- The window's rev is its LATEST calendar day, taken from the padded day list --
  so a record's rev can be a day on which that entity did not change (padding
  makes the last day exist even if empty). This is the point of padding; verified
  in TestApply_PadsAndSkipsEmptyWindows (commit on 01-01, period 2 -> rev 01-02).
- Dedupe by entity keeps the EARLIEST occurrence within the window (days iterated
  ascending, `group-by` preserves per-day log order); only Rev is overwritten,
  all other fields (Author/Date/Loc) stay from that earliest record. Date is
  intentionally NOT rewritten to the window date -- the ticket only specifies Rev.
- Empty windows need no explicit skip: mergeWindow returns nil for a window whose
  days are all empty, and appending nil is a no-op.
- Error taxonomy (consistent with cod-kyfe rule -- classify by source of bytes):
  period<1 is a USAGE error (`invalid_temporal_period`, exit 2, from a flag);
  a non-YYYY-MM-dd Date is an INPUT error (`invalid_temporal_date`, exit 3, from
  the log). Dates parsed with time.ParseInLocation(..., time.UTC) for
  reproducibility (matches the UTC rule the design sets for code-age).
- `.local/refs/code-maat` is a BROKEN symlink in this environment (points at a
  path on the original author's machine), so time_based_grouper_test.clj could
  not be ported byte-for-byte. The "ported fixture" case is a representative
  padding+empty-window-skip scenario grounded in research §5.2 and the ticket
  algorithm. Same corpus-unavailable situation reported by the coupling/group
  tickets -- the corpus is simply not present here.

## P3-4 transform/teammap (cod-2boa)

- Mapping-input error taxonomy diverges by design intent, not by source of bytes:
  `group` classifies a malformed `--group` as a USAGE error
  (`invalid_group`, exit 2) while `teammap` classifies a malformed `--team-map`
  as an INPUT error (`invalid_team_map`, exit 3). Both come from flag-referenced
  files, so this is not the "who produced the bytes" rule -- it follows
  cli-design §7.2, which lists malformed `--group`/`--team-map` under exit 3. The
  ticket for teammap pins exit 3 explicitly; group's exit 2 predates and is left
  as-is. If these are ever unified, exit 3 is the design-authoritative value.
- CSV header detection is positional + literal: only a FIRST row equal to
  `author,team` (case-insensitive, trimmed) is treated as a header. A headerless
  file therefore Just Works, and a genuine author literally named "author" on
  team "team" as the first row would be misread -- acceptable given the fixed
  two-column schema and matching the ticket's "if first row is literally
  author,team treat as header".
- encoding/csv with FieldsPerRecord=2 turns a wrong-column-count row into the
  parse error we want for free (no manual len check per row).
- JSON supports BOTH the object form `{"author":"team"}` (primary) and the array
  form `[{"author","team"}]` (symmetry with group), dispatched on the first
  non-space byte. Same corpus-unavailable situation as sibling tickets:
  team_mapper_test.clj could not be ported byte-for-byte (broken
  `.local/refs/code-maat` symlink), so TestApply_PortedFixture is a representative
  remap+passthrough scenario grounded in research §5.3 (APN/XYZ->Blue,
  ZOP->Yellow, unmapped QQQ kept).
- Apply copies the input slice before remapping (unlike group.Apply, which builds
  a filtered slice anyway); this preserves the "does not mutate input" contract
  that P3-1 pipeline relies on when chaining transforms.

## P3-1 pipeline (cod-0xx4)

- The pipeline is the single place transforms compose; order is fixed
  group -> temporal -> teammap (research §4, matching `parse-commits-to-dataset`).
  Grouping first matters: it renames entities before temporal dedup-by-entity and
  before team metrics, so a stage's output is what the next stage sees.
- Stage-skip guards are zero-value checks, but slices/maps use `len>0`, NOT
  `!= nil`: an EMPTY `group.Spec` set must mean "no grouping requested" (skip),
  because passing it to `group.Apply` would drop every entity (nothing matches).
  `nil=skip` in the ticket really means "nil or empty = skip".
- Two distinct error classes meet here and must stay separate: a malformed
  definition keeps each transform's own coded error (group=exit 2, teammap=exit 3
  -- see P3-4 note), while an unreadable --group/--team-map PATH is a new
  `errFileOpen` (input_error, exit 3, details {flag,path}), mirroring errLogOpen.
  File-open failure != definition-parse failure.
- Transforms are applied UNIVERSALLY to every analysis (the global flags live on
  all subcommands), including author/count analyses where temporal windowing is
  semantically wrong (research §5.2 warns this). We match code-maat: apply and let
  the user choose, rather than gate per analysis.
- Testing temporal end-to-end without a coupling analysis: a same-day double
  commit to one entity collapses under `--temporal-period 1` (one window/day,
  dedup-by-entity keeps the earliest), giving an observable n_revs drop on
  `authors`. Cross-day collapse needs period >= the span.
- Reused the same-package `authorsEnvelope` type from commands_test.go for the
  e2e assertions; definition files written to t.TempDir() keep the pipeline e2e
  hermetic (log via stdin, --group/--team-map via temp files), no committed goldens.

## P4-1 revisions (cod-0sst)

- CRITICAL divergence from authors: revisions NRevs = `len(calc.Distinct(revs))`
  (distinct Rev per entity), NOT `len(group.Items)` (row count) the way authors
  computes its own n_revs. Same JSON key `n_revs`, different definition, because
  authors mirrors the original's revisions-in=nrows shorthand while revisions is
  a true distinct-rev count (research §6). A rev touching an entity twice counts
  once here, twice in authors.
- Sort n_revs desc, entity asc as the final deterministic tiebreak (same pattern
  as authors); the original sorts by n-revs desc only, entity tiebreak added for
  reproducible --rows truncation. GroupBy already yields entity-asc groups so
  sort.SliceStable is enough.
- Straight copy of the authors slice: descriptor via init()+Register, row struct
  with json tags, Run builds the full envelope. No corpus fixture needed; the
  three research-grounded cases (distinct count, sort/tiebreak, empty) fully pin
  the algorithm.

## P4-8 absolute-churn (cod-asdr)

- Resolved the P4-7 tension (analyses in `analysis` cannot reach unexported
  churn helpers) by EXPORTING rather than co-locating: `requireLoc`/`sumByGroup`/
  `groupChurn` became `RequireLoc`/`SumByGroup`/`GroupChurn`. This matches the
  house style of the other helper subpackages (`effort.ByEntity`,
  `couplingalgo.Couplings`) and keeps the analysis in `internal/analysis/` as the
  ticket specified. The two remaining unexported churn helpers
  (`byEntityAuthorContrib` and friends) are left alone; export them when
  entity-ownership/main-developer wire them (surgical, not speculative).
  Downstream author-churn/entity-churn just reuse `RequireLoc`+`SumByGroup` with
  a different key func and sort - no new churn code needed.
- Sort is `[date, added, deleted]` asc per research §6, but dates are the group
  key so they are already distinct and asc out of `SumByGroup`; the loc
  tiebreakers can never fire. Kept the full comparator anyway to mirror the
  original's order-by verbatim and stay robust if the grouping ever changes.
- The `missing_metrics` guard only fires for parsed mods that carry no loc; a
  message-only log with no numstat lines parses to ZERO mods (empty result, ok,
  exit 0), not missing_metrics. Unit-tested the guard directly with
  `HasLoc:false` mods rather than via the CLI, which cannot easily produce
  loc-less-but-present rows from the git2 parser.

## P4-20 analysis: messages (cod-yo7b)

- `errors.Is(err, ErrInvalidExpression)` does NOT work when the error was built
  with `.Wrap(...)`/`.WithDetails(...)`: those return a _copy_ of the sentinel
  (`c := *e`) with no custom `Is`, so pointer identity is lost. Assert coded
  errors the way `transform/group` does: `errors.As(err, &coded)` then compare
  `coded.Code()`/`coded.ExitCode()`. `errors.Is` only holds for sentinels
  returned verbatim (e.g. `ErrMissingMessages`, which `requireMessages` returns
  unmodified).
- Sort is `[matches, entity]` DESC on BOTH columns (faithful to code-maat's
  single-direction `-order-by ... :desc`), so entity ties break _descending_ -
  unlike authors/revisions which pin entity ascending. It is still fully
  deterministic because `entity` is the group key (unique per row).
- `matches` counts distinct revisions among matching mods (`calc.Distinct` on
  Rev), matching code-maat's revisions-by-entity rename to `matches`; equals row
  count for a normal git2 log where each entity appears once per commit.
- missing-vs-invalid expression both exit 2: at the CLI the required `--expression`
  flag is enforced by urfave (`missing_required_flag`, see P5-4) before Run; the
  analysis's own empty-expression guard returns `invalid_expression` for the
  direct unit-test path. A message-less log (every `Message=="-"`) is `missing_messages`
  (exit 3); an empty mod slice is not an error (empty_log is the parser's job).

## P4-12 analysis: main-developer (cod-x5r6)

- The per-(entity, author) contribution helper existed in `churn` but was
  unexported and only used by its own package test (`byEntityAuthorContrib`).
  main-developer lives in package `analysis`, so I exported it as
  `churn.ByEntityAuthorContrib` (+ `EntityContribs`/`AuthorContrib`). It returns
  both levels in ascending key order, which is what makes the max-adder tiebreak
  work: iterate and keep the first author on an equal `added`, so ties resolve to
  the lexicographically smallest author with no extra sort. entity-ownership
  (cod-rq5s) and refactoring-main-developer (cod-7w03, ranks by deleted) can
  reuse the same helper.
- `ownership` is `calc.CentiRatio(mainAdded, total)` = two SIGNIFICANT digits,
  not two decimals: 164/245 = 0.6693 -> 0.67, and 15/16 = 0.9375 -> 0.94.
  `CentiRatio` already guards total<1 -> denom 1, so a binary-only entity
  (total_added 0) yields ownership 0.0 rather than dividing by zero.
- No cmd/schema wiring needed: the command, the `main-dev` alias, and
  `schema --command main-developer` are all generated from the registered
  descriptor. Verified end-to-end with a piped git2 log and the schema command.

## P4-3 analysis: parse / identity (cod-ghn2)

- `parse` is a passthrough dump: it emits the parsed `[]Modification` verbatim
  in LOG ORDER with no filtering, aggregation, or sorting. It runs after the
  pipeline's group/temporal/team-map transforms (so it reflects the transformed
  set), before any analysis - the debug/interop escape hatch of design 6.4.
- Conditional loc fields use `*int` + `omitempty`, not `int`: a record without
  numstat (`HasLoc=false`) omits `loc_added`/`loc_deleted` entirely rather than
  reporting a misleading `0`. Take the address of a local copy of the loop
  variable's fields, not `&m.LocAdded`, to be safe across Go versions. `binary`
  is `bool,omitempty` (present only when true). In CSV/table a missing key
  renders as an empty cell (`cellString(nil)`), so absent loc shows blank while
  a real `0` (binary text row) shows `0`.
- Asserting omitempty behaviour: marshal the row to JSON and check the key is
  absent from the decoded map - testing the pointer being nil alone does not
  prove the wire contract.
- No cmd/schema wiring needed: the command, the `identity` alias, and
  `schema --command parse` are all generated from the registered descriptor.
  `empty_log` is the parser's job upstream, so `runParse(nil,...)` is a clean
  empty result, not an error.

## P4-11 analysis: entity-ownership (cod-rq5s)

- One row per (entity, author) of `{entity, author, added, deleted}`. Reuses
  `churn.ByEntityAuthorContrib` (built for exactly this) and `churn.RequireLoc`
  for the `missing_metrics` guard (exit 3) shared with the churn family.
- Determinism without a compound sort key: the helper yields entities in
  ascending order and, within each entity, authors in ascending order. A single
  `SliceStable` by entity asc preserves that inner author order, so ties break
  by ascending author for free (matches the original's `entity` asc sort).
- No cmd/schema wiring needed: command and `schema --command entity-ownership`
  are generated from the registered descriptor. Verified e2e (json/csv/schema).

## P4-16 analysis: main-developer-by-revisions (cod-ew96)

- The effort family (`effort.ByEntity`) counts revisions as rows, so this
  analysis needs no loc metrics: its only error code is `empty_log` (handled
  upstream), NOT `missing_metrics`. This is the key contrast with the
  churn-based `main-developer`, which requires loc and can exit 3.
- Same first-on-tie shortcut as entity-ownership: `effort.ByEntity` returns
  authors ascending, so scanning for max `Revs` with strict `>` keeps the
  lexicographically-first author on a tie for free - no compound sort key.
- Column names `added`/`total_added` hold rev counts, not lines: they mirror
  the original's column names for parity even though the effort family measures
  revisions. Ownership via `calc.CentiRatio` (2 sig digits), sort entity asc.

## P4-17 analysis: fragmentation (cod-6xou)

- Reuses `effort.ByEntity` (rows-as-revisions, so `empty_log` is the only error
  code, no `missing_metrics`). Fractal value per entity is
  `1 - Σ(author_revs/total_revs)²`, computed with float64 (rounding to 2 sig
  digits absorbs the small fp error vs the original's exact rationals).
- The fractal value is an already-computed float, not an own/total ratio, so
  `calc.CentiRatio` did not fit. Extracted the 2-sig-digit rounding into
  `calc.CentiFloat(float64) float64` and made `CentiRatio` a thin wrapper over
  it. `communication` (P4-18) will want the same helper for its strength math.
- Sort is `[fractal_value, total_revs]` DESC (a real compound key this time,
  unlike the effort analyses that sort by a single ascending key). `SliceStable`
  keeps entity-ascending order for fully tied rows, matching the original.

## P4-19 analysis: code-age (cod-qmk4)

- Whole-month diff matches clj-time `in-months`: `months = (now.Y-from.Y)*12 +
(now.M-from.M)`, minus 1 when `now.Day < from.Day` (final month not yet
  elapsed). Verified against the ticket's cases (e.g. 2020-01-15 -> 2020-02-10
  is 0 months, not 1).
- "Strictly before now": a change dated exactly on `--time-now` is excluded; an
  entity whose only changes are on/after now is dropped from the output entirely
  rather than emitted with a zero/negative age.
- Dates parse in UTC via `time.Parse("2006-01-02", ...)` (that layout yields UTC
  already); empty `--time-now` resolves to today's UTC calendar date, tested via
  `resolveNow("")` directly to stay deterministic without magic month counts.
- `--time-now` was already wired to `Opts.TimeNow` in commands.go (declared-flag
  gate), so only the descriptor's `Flags` entry was needed to activate it.
- ErrorCodes lists `invalid_time_now` (exit 2) in addition to `empty_log`,
  following the messages/`invalid_expression` precedent: an analysis that
  validates a flag surfaces that usage error in its own schema for accuracy,
  even though the ticket Design sketch showed only `empty_log`.

## P4-18 analysis: communication (cod-90c9)

- Ported from code-maat `communication.clj`: per entity take the distinct
  authors (via `effort.ByEntity`), form all ordered pairs-with-replacement
  (`selections`), and count entity co-occurrences into a frequency map. The
  self-pair `freq[a,a]` is the number of distinct entities author `a` touched.
  The ticket calls this "total commits for a", but it is entities-touched, not
  commit count; the strength formula only cares that it is `freq[me,me]`.
- Per directed pair `me != peer`: `average = Ceil(Average(freq[me,me],
freq[peer,peer]))`, `strength = TruncInt(Percentage(shared/average))`. Both
  directions are emitted (co-occurrence is symmetric, so the two rows differ
  only in subject); self-pairs feed the counts but are excluded from output.
- Contrary to the fragmentation note's guess, communication does NOT use
  `calc.CentiFloat`: strength is a truncated integer percentage, so it reuses
  `Ceil`/`Average`/`Percentage`/`TruncInt` like coupling, not the 2-sig-digit
  rounding.
- Sort is `[strength, author, peer]` all DESC. The original does
  `reverse(sort-by [strength author])` and leaves full ties to nondeterministic
  map order; `peer` DESC is added as a deterministic final tie-break.
- No code-maat `.clj` fixtures are vendored in the repo, so the tests are
  hand-derived from the algorithm rather than pinned to the original corpus.
  If the corpus is later imported, add a golden e2e in the authors-style suite.

## P5-4 cmd: usage-error classification -> exit 2 (cod-ic8y)

- urfave/cli v3 usage errors were previously collapsed to a single
  `usage_error` code. `classifyUsageError` now maps the framework's message
  substrings to distinct codes: `flag provided but not defined`/`no such flag`
  -> `unknown_flag`; `invalid value` -> `invalid_value`; `Required flag`/`not
set` -> `missing_required_flag`. Unknown _commands_ are classified upstream in
  `run()` (via `CommandNotFound`), now coded `unknown_command`. All stay exit 2.
- Ordering in the `usageClasses` table matters: first matching marker wins, so
  specific markers precede general ones (none currently overlap, but the guard
  is intentional).
- Key discovery: by default urfave prints an "Incorrect Usage: ..." banner to
  ErrWriter AND a full command-help dump to Writer (stdout) on any flag-parse or
  missing-required-flag error, polluting stdout. Setting a command's
  `OnUsageError` hook to a passthrough (`return err`) suppresses BOTH (see
  urfave `command_run.go` lines ~189/348: the `OnUsageError != nil` branch skips
  the Fprintf + ShowSubcommandHelp). The hook is NOT inherited, so it must be set
  on the root and every subcommand (analysis commands, print-log-command,
  schema). Returning the raw error is safe: the root's no-op `ExitErrHandler`
  short-circuits `handleExitCoder` so no os.Exit fires, and `run()` then codes
  the error centrally.
- The other `usage_error` codes in the tree (invalid `--after`, unknown schema
  command, invalid `--format`) are deliberate application-level coded errors, a
  separate category from framework parse errors; they were left unchanged.

## P5-2 cmd: version subcommand + --version (cod-7fb5)

- Two version surfaces share one source, `internal/version.Current()`: the root
  command's `Version` field (drives urfave's built-in `--version`, which prints
  `codelens version <v>`) and the new `version` subcommand
  (`cmd/codelens/version_cmd.go`), which prints the bare version string (plain,
  no envelope) for easy capture. Both were verified against the ldflags-stamped
  build (`--version` and `version` reported the same value).
- Fixed a latent schema drift: `schema.go` `metaSummaries()` already advertised
  a `version` command (declared as "wired in a later phase"), but no such
  subcommand existed, so `codelens version` returned `unknown_command` (exit 2)
  while `codelens schema` claimed it existed. Wiring the subcommand and updating
  the `versionUsage` comment closes the gap. Lesson: the meta-command list in
  `metaSummaries()` is hand-maintained (unlike analyses, which are registry-
  driven), so entries there must land in the same change as the command itself.

## P5-3 cmd: error/exit code registration + conformance (cod-qoff)

- The output command registry (`internal/output/registry.go`,
  `RegisterExitCodes`/`ExitCodesFor`) was built in P0-5 but never called: dead
  infrastructure. This ticket wired it. `cmd/codelens/exitcodes.go` populates it
  at package `init()` from two sources: analyses via each descriptor's
  `ExitCodes`, and the meta commands via `metaSummaries()`. Registering at init
  (rather than inside `analysisCommands`, which `run()` calls per invocation)
  means the registry is populated the moment the binary loads, independent of
  any `run()` call.
- Error codes are deliberately NOT put in the registry: there is no
  error-code registry function, and the codes already live on the descriptor and
  reach the schema via `analysis.Schema`. Keeping one source per analysis avoids
  drift. The registry is purely a name-keyed exit-code cache for consumers that
  hold a command name but not its descriptor.
- The schema command still reads exit/error codes straight from the descriptor
  (`analysis.Schema(d)`) - reading the registry would be pointless indirection
  since the builder already has the descriptor. The conformance test guarantees
  the descriptor, the registry, and the emitted schema all agree, so the
  denormalized cache can never silently diverge.
- Conformance test placement matters: the `analysis` package's own tests call an
  unexported `resetRegistry()` that wipes `analysis.All()` and never restores it,
  so a test relying on the full registered set is fragile there (order-dependent
  on sibling test files). The `cmd/codelens` test binary is a separate process
  with a fully init'd registry that nothing resets - that is where
  `TestExitCodesRegistered_AllCommands` lives, and it also matches the
  `schema --command CMD` end-to-end framing.

## P6-1 AGENTS.md (cod-tyux)

- Docs must track the shipped surface, not the design doc. cli-design.md 6.1
  specifies a `params` field echoing effective tuning options in the JSON
  envelope, and `internal/output/types.go` defines `Params map[string]any` with
  `omitempty`, but no analysis populates it - so `params` never appears in real
  output. AGENTS.md documents the envelope as actually emitted
  (`schema_version, ok, analysis, row_count, [total_count, truncated], rows`)
  and omits `params`. If a future ticket wires params, update AGENTS.md too.
- Verify command/flag names against `codelens schema` (the registry), not memory
  or the design tables: `authors` has no `--min-revs` flag despite cli-design.md
  4.3 grouping it under a shared `--min-revs` row; the per-command schema is the
  source of truth for which flags a given analysis actually accepts.

## P6-4 CLAUDE.md (cod-8831)

- The ticket described CLAUDE.md as a stub (`codelens is .`), but the `init`
  commit had already turned it into a symlink to `AGENTS.md`. That made
  AGENTS.md's line "For build and contribution instructions, see `CLAUDE.md`"
  circular. plan.md P6-4 and its exit criteria distinguish the two: AGENTS.md is
  the operating/agent guide, CLAUDE.md is the build/contribution guide. Resolved
  by replacing the symlink with a real CLAUDE.md (description + project layout +
  Makefile-target table + specs pointer + pointer back to AGENTS.md).
- The Makefile is the source of truth for the build/validation table; verify
  target names against it rather than the design docs (`validate` = fmt-check +
  vet + lint + test; `build` = validate + clean-local + build-local).

## Deepen result envelope (cod-ikw0)

- The success envelope was rebuilt at 19 call sites, each hard-coding its own
  analysis name as a literal duplicating `Descriptor.Name`. Narrowing
  `Descriptor.Run` to `(any, error)` (rows only) and adding `output.NewResult`
  concentrated the envelope invariants in one place and sourced the name from
  `d.Name` in the dispatcher, so the literals could not drift.
- `Result.Params` had been declared-but-dead since P6-1. `actionFor` is the only
  place that knows both the descriptor and the effective flag values, so it owns
  populating params via `effectiveParams(cmd, d)`. Returning `nil` for flagless
  analyses (rather than an empty map) keeps `omitempty` working, so their output
  stays byte-identical - only the four flag-bearing analyses (coupling,
  sum-of-coupling, code-age, messages) gain a `params` object.
- Some in-package test helpers accessed `res.OK`/`res.Rows` on a value whose type
  (`output.Result`) came from another package without importing it: Go needs the
  import only to _name_ a type, not to access fields of an inferred value. After
  `Run` returned `any`, those helpers had to type-assert the rows explicitly, and
  the `output` import fell away from every analysis run file and most test files
  (only `schema.go`/`schema_test.go` keep it, for the unrelated schema envelope's
  `output.SchemaVersion`).
- Single reflection site: `output.RowLen(any) int` guards `Kind()==Slice` and is
  reused by both `NewResult` (row count) and the cmd-layer `truncate` (cap guard);
  truncate still needs its own `reflect.ValueOf(...).Slice` for the actual slice,
  which only runs once `RowLen` has confirmed a non-empty slice.
- Envelope invariants are now asserted once (`output/newresult_test.go`) plus one
  dispatch-level params test in `cmd/codelens`; per-analysis tests assert rows
  only. The P6-1 note's warning held: wiring params meant updating the operating
  reference (`docs/skills/codelens/references/operating.md`) to document the new
  `params` object.

## Concentrate aggregation loops into calc helpers (cod-ya2e)

- Three analyses (maindev, refmaindev, maindevbyrevs) ran an identical
  max-contributor reduce differing only by the scored field. Extracting
  `calc.MaxBy(items, val) (top, total)` put the tie-break rule (strict `>`, so a
  pre-sorted-by-author input keeps the first author on a tie) and the running
  total in one place. `Map`/`FlatMap` were added alongside for the churn trio and
  flatten pair; they concentrate no logic, only shape, and are kept because a
  closure reads no worse than the `for` loop here.
- Parity trap: `maindevbyrevs` must keep reporting `top.TotalRevs`, not `MaxBy`'s
  returned sum. `effort.AuthorRevs.TotalRevs` is the entity-wide total repeated on
  every author row, so summing it over authors would multiply the total by the
  author count. `MaxBy`'s `total` is correct for the churn reduces (per-author
  added/deleted, disjoint) but wrong for the revs reduce (total pre-broadcast).
- `Map`/`FlatMap` return `make([]R, 0, len(src))` so an empty input yields a
  non-nil empty slice, matching the accumulator they replace; this keeps empty
  results marshaling to `rows: []` rather than `null`. Tests pin the non-nil
  contract directly.
- Pure internal refactor: the existing per-analysis and CLI golden tests were the
  parity guard and passed byte-identical with no expectation changes. No
  Descriptor, RowSchema, row struct, or sort order touched.

## Bring meta commands onto the descriptor spine (cod-451g)

- Meta commands (`schema`, `version`, `print-log-command`) now share one source
  each: a `metaCommand` struct + `metaCommands()` table in `cmd/codelens/metacommands.go`
  projects to the cli wiring (`command()`), the schema command list (`summary()`),
  and the full introspection schema (`schema()`). Previously Name/Summary/ExitCodes
  were declared twice (in each builder func and in `metaSummaries()`); the table
  removes that drift surface.
- Meta commands stay a distinct type, not an `analysis.Descriptor`: a Descriptor's
  `Run(mods, opts)` and `RowSchema` model log-in/rows-out, which meta commands do
  not have. `analysis.MetaSchema(command, summary, flags, errorCodes, exitCodes)`
  builds their `CommandSchema` from explicit parts, forcing `Aliases`/`RowSchema`
  to empty `[]`; `Schema(d)` for analyses is untouched.
- New capability (not a regression): `schema --command schema|version|print-log-command`
  returns a `CommandSchema` (exit 0) instead of `usage_error`. No prior test
  asserted the old error. The unknown-`--command` hint's `known_commands` now
  merges analysis + meta names via `allCommandNames()` (sorted), so it stays
  complete.
- Flags reuse `toCLIFlag` from the `analysis.Flag` shape, so a meta flag's Usage
  text comes from its `Desc` exactly as for analyses. `metaCommand.command()`
  wiring and `metaCommand.schema()` therefore cannot disagree on the flag set;
  a conformance test (`TestMetaCommands_SchemaFlagsMatchWiredFlags`) pins that.
- The candidate-2 exit-code conformance guard (`schemacodes_test.go`) now iterates
  `metaCommands()` directly and checks each meta command through BOTH the command
  list and `schema --command`, replacing the old list-only check over
  `metaSummaries()`.
