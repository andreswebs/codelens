---
id: cod-fb72
status: open
deps: []
links: []
created: 2026-07-27T12:59:52Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-z4wu
tags: [codelens, spec-002, output, breaking]
---
# Remove --format and the alternate serializers (JSON only)

Remove the `--format` flag and the ndjson/csv/table serializers, leaving JSON as the one
output representation. This is the first of the ADR 0008 rollout tickets and is
deliberately PURELY SUBTRACTIVE: no envelope key is added here, so the golden diff is a
clean deletion and is reviewable on its own. `shape` and `semantics` arrive in the two
follow-on tickets.

Implements decision D2 from docs/specs/002-data-output/plan.md section 2: no
transitional error. After removal, `codelens --format json authors` is classified by the
existing usage classifier as `unknown_flag` (exit 64), which is exactly what a removed
flag means. It is pinned by one new golden so a re-introduction fails the suite.

Breaking change: any consumer of `ndjson`, `csv`, or `table` breaks. That is the point.
The code-maat CSV compatibility (kebab-case headers) is cut deliberately, freeing column
naming from an external contract. Pre-1.0, so no deprecation cycle.

Reference: docs/specs/002-data-output/plan.md section 4 (this ticket's step list),
section 1 (the measured format surface), docs/adr/0008-canonical-output-representation.md.
Skills: /golang, /llm-coding (surgical, verifiable).

## Design

All line numbers were verified at 2026-07-26. Re-locate by grep if they have drifted.

### 1. Delete the serializer file and its test

Delete `internal/output/format.go` and `internal/output/format_test.go` outright. The
file holds `Emit`, `emitNDJSON`, `emitCSV`, `emitTable`, `rowMaps`, `cellString`,
`snakeToKebab`, `rowObjects`, and the `errUnknownFormat` sentinel. Nothing outside
format.go calls `rowObjects` (verified: `grep -rn 'rowObjects\|rowMaps\|cellString'`
returns hits only in format.go and format_test.go), so the whole file goes.

`output.Emit`'s only production caller is `internal/command/commands.go:175`. It
collapses to `output.EmitProjected`, which already exists in `internal/output/fields.go:65`
and already handles the `--fields` projection plus the plain-JSON path.

### 2. `internal/command/commands.go`

- Delete the `--format` `cli.StringFlag` at lines 38-43 (`globalFlags`).
- Change `globalFlags(format *string) []cli.Flag` at line 27 to `globalFlags() []cli.Flag`
  and drop the `Destination: format` binding.
- Line 175, in `actionFor`:

```go
// before
return output.Emit(cmd.Root().Writer, cmd.String("format"), res, columnNames(d), cmd.String("fields"))
// after
return output.EmitProjected(cmd.Root().Writer, res, cmd.String("fields"))
```

- Delete `columnNames` (lines 235-243). Its doc comment names the csv/table formats as
  its only purpose. NOTE for the next ticket: `columnNames` is revived in the `shape`
  ticket for a different purpose (seeding the `--fields` valid-path set and populating
  `output.Meta.Columns`), so expect to write it again; do not try to keep it here on that
  basis, since a dead function would fail lint.
- Also update the `--fields` flag Usage at line 46: `"comma-separated JSON field PATHS to
  project (json only)"` loses the `(json only)` qualifier, since there is no longer a
  format to qualify it.

### 3. `internal/command/root.go`

- Delete `var format string` (line 27) and change the `Flags: globalFlags(&format)`
  wiring (line 37) to `Flags: globalFlags()`.
- The doc comment on `globalFlags` in commands.go (lines 21-26) explains that format is
  bound to the caller's destination "so the top level always has a valid output format
  for EmitError even when flag parsing fails before a subcommand's Action runs". That
  whole rationale disappears with the flag; rewrite the comment to describe only why the
  flags are registered on the root rather than Local (urfave inherits them into each
  subcommand's flag set and resolves them via the command lineage, so a global flag works
  before or after the subcommand name).

### 4. `internal/analysis/schema.go`

- Remove `"unknown_format"` from `commonErrorCodes` (line 49). The list is
  alphabetically ordered; keep it so.
- Update the derivation comment above it (lines 15-33): the sentence listing "every
  global-flag/output code (unknown_format, invalid_field, ...)" must drop
  `unknown_format`. That comment is load-bearing: a conformance test pins every entry to
  a terr sentinel, so a stale name would be misleading.

### 5. `internal/command/registry_guard_test.go`

- `wantSentinelCount` (line 32) goes from 22 to 21.
- Remove the `internal/output/format.go   1 (unknown_format)` line from the per-file
  derivation tally in the comment (line 17), and update the "per-file tallies sum to 22"
  sentence to 21.

### 6. `internal/command/error_format_test.go`

The file's single test, `TestError_FormatText_StillJSONEnvelope`, loops over
`--format text|table|json` to prove errors are always the JSON envelope on stderr. The
premise (a format flag exists) is gone. Replace it with the same assertion driven by a
plain forced error, and rename the file to `internal/command/error_envelope_test.go`:

```go
// TestError_IsAlwaysJSONEnvelope pins the always-JSON error decision: there is no
// human-facing text error path, so a forced data error yields the JSON error envelope on
// stderr and nothing on stdout. An empty stdin forces empty_log (exit 65).
func TestError_IsAlwaysJSONEnvelope(t *testing.T) { ... }
```

Keep the existing assertions: exit 65, stderr parses as an error envelope with
`ok: false` and a non-empty `code`/`message`, stdout empty, and no `✗` character (the
removed text path's marker).

### 7. `internal/command/golden_test.go`

Delete six scenarios from the `goldenCases` table (lines ~92-118):

```go
{"authors_ndjson", []string{"--format", "ndjson", "authors"}, authorsLog},
{"authors_csv",    []string{"--format", "csv", "authors"}, authorsLog},
{"authors_table",  []string{"--format", "table", "authors"}, authorsLog},
{"format_error_text",  []string{"--format", "text", "authors"}, ""},
{"format_error_table", []string{"--format", "table", "authors"}, ""},
{"format_error_json",  []string{"--format", "json", "authors"}, ""},
{"unknown_format", []string{"--format", "bogus", "authors"}, sampleGitLog},
```

(that is the three success variants, the three identical error variants, and
unknown_format: seven entries, six scenario names plus unknown_format).

Add two:

```go
// empty_log: keeps exit-65 and empty_log coverage, which the deleted
// format_error_* scenarios were incidentally providing (all three were the same
// empty_log envelope).
{"empty_log", []string{"authors"}, ""},

// removed_format_flag: pins that --format is gone. Without this, a
// re-introduction of the flag would pass the suite (D2).
{"removed_format_flag", []string{"--format", "json", "authors"}, authorsLog},
```

Update the coverage comment block (lines ~57-67). It currently reads:

```go
//   - 0  the seven authors success variants, the coupling warning, schema list
//   - 64 the four usage errors, unknown_format, unknown_schema_command,
//     invalid_after_date
//   - 65 the three --format data errors, invalid_control_char
```

The 0 line becomes four authors variants (json, fields, rows2, schema); the 64 line drops
`unknown_format` and gains `removed_format_flag`; the 65 line becomes `empty_log` plus
`invalid_control_char`. Also check the `sampleGitLog` comment at lines ~78-80, which
names `unknown_format` as a reason a valid log is needed on stdin; `removed_format_flag`
inherits that need (an empty stdin would short-circuit to empty_log before the flag is
rejected), so retarget the comment rather than deleting it.

### 8. Regenerate and prune goldens

```sh
go test ./internal/command/ -run TestGolden -update
```

Then delete the 18 orphaned files by hand (`-update` regenerates existing scenarios; it
does not remove goldens whose scenario is gone):

```text
testdata/authors_{ndjson,csv,table}.{out,err,exit}      9 files
testdata/format_error_{text,table,json}.{out,err,exit}  9 files
testdata/unknown_format.{out,err,exit}                  3 files
```

That is 21 deletions, and 6 additions (`empty_log.*`, `removed_format_flag.*`). Review
every diff by hand: the goldens are the frozen output contract, per ADR 0007, and a
surprising diff means something else moved. `authors_json`, `authors_fields`,
`authors_rows2`, `authors_schema`, `schema_list`, and `coupling_warning` must come back
BYTE-IDENTICAL, since nothing in this ticket changes their output. If `authors_schema` or
`schema_list` moves, it is because of the `unknown_format` removal from
`common_error_codes` and the errors inventory, which is expected and correct.

### 9. `internal/command/testdata/README.md`

- The intro says the goldens freeze "every format, `--fields`, `--rows`, ..." and
  mentions "the renamed I/O and format error codes". Restate without the format matrix.
- The "Golden naming scheme" section lists example scenario names including
  `authors_json`; that name survives, so only prune what no longer exists.

### Verification sweep

```sh
grep -rn -- '--format' internal/ cmd/ | grep -v 'group-format\|team-map-format'
grep -rni 'ndjson' internal/ cmd/
```

Expect zero hits in `internal/` and `cmd/` except: the two surviving `*-format` flags
(`--group-format`, `--team-map-format`, which are input-parsing selectors and stay), and
`internal/output/warning.go` if it documents that warnings form an NDJSON stream on
stderr (that is stderr diagnostics, unrelated to the results format, and stays).

### Out of scope

- Any envelope key addition (`shape`, `semantics`, `transforms`): the next two tickets.
- Docs outside `internal/command/testdata/README.md`: the docs ticket. README.md,
  docs/cli-design.md, and the skill keep their stale `--format` references until then;
  this ticket does not need them to be consistent to land.
- `--fields` / `--rows` behaviour: unchanged here.

### Files touched

```text
internal/output/format.go                      DELETE
internal/output/format_test.go                 DELETE
internal/command/commands.go                   drop --format flag, EmitProjected call, drop columnNames, --fields usage text
internal/command/root.go                       drop format var and binding
internal/analysis/schema.go                    drop unknown_format from commonErrorCodes + comment
internal/command/registry_guard_test.go        sentinel count 22 -> 21, comment tally
internal/command/error_format_test.go          DELETE, replaced by error_envelope_test.go
internal/command/error_envelope_test.go        NEW
internal/command/golden_test.go                table + coverage comment
internal/command/testdata/*                    21 deletions, 6 additions
internal/command/testdata/README.md            drop format framing
```

## Acceptance Criteria

- `internal/output/format.go` and `format_test.go` no longer exist; `output.Emit` is gone
  and `internal/command/commands.go` emits via `output.EmitProjected`.
- No `--format` flag exists. `codelens --format json authors` exits 64 with an
  `unknown_flag` error envelope on stderr and nothing on stdout, frozen by the
  `removed_format_flag` golden.
- `codelens authors` output is byte-identical to before this change; the `authors_json`,
  `authors_fields`, `authors_rows2`, and `coupling_warning` goldens are unchanged.
- `unknown_format` is absent from `commonErrorCodes`, from the `schema` errors inventory,
  and from every `common_error_codes` list in schema output. `wantSentinelCount` is 21 and
  `TestRegistry_Coherent` passes.
- Errors are still always the JSON envelope on stderr, asserted by
  `internal/command/error_envelope_test.go` without reference to any format.
- Golden coverage still exercises every reachable exit code (0, 64, 65, 74), with exit 65
  covered by the new `empty_log` scenario; `TestExitCodes_DeclaredCodesAreExercised`
  passes.
- `grep -rn -- '--format' internal/ cmd/` returns only `--group-format` and
  `--team-map-format`.
- No orphaned golden files remain in `internal/command/testdata/` (no `.out`/`.err`/`.exit`
  without a scenario in the `goldenCases` table).
- `make build` green.

