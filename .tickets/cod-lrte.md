---
id: cod-lrte
status: open
deps: []
links: []
created: 2026-07-15T03:40:57Z
type: feature
priority: 1
assignee: Andre Silva
tags: [codelens, cli, feature, friction]
---
# Feature: structured JSON diagnostics on stderr

Make `codelens`'s stderr diagnostics fully machine-readable: (1) errors are
**always** emitted as the JSON error envelope on stderr, regardless of `--format`;
and (2) add a structured JSON **diagnostic/warning** line on stderr that non-fatal
advisories emit. This is the shared channel the grouped-coupling warning ticket
(`cod-btfg`) depends on.

## Current behavior

`src/internal/output/errors.go`:

- `EmitError(w, format, err)` renders the JSON error envelope
  (`{schema_version, ok:false, error:{code,message,hint,details}}`) for every
  format **except** `text`, where `render()` returns `✗ <message>` plus an
  optional `hint:` line.
- `main.go` calls `output.EmitError(stderr, format, err)` for any returned error
  and, only under `--debug`, also writes a JSON slog line.

So errors are already JSON on stderr for `json`/`ndjson`/`csv`, but `--format
text` (and `table`) suppress the envelope, and there is no channel at all for
non-fatal warnings.

## Decision

- **Errors: always JSON (approved decision).** Drop the format branch for errors;
  the JSON error envelope is the sole error rendering on stderr for **all**
  `--format` values, including `text` and `table`. The `--format` flag governs the
  **results** on stdout, not diagnostics on stderr. This is an accepted, intended
  behavior change: the human-facing `✗ <message>` text error path is **removed
  entirely** (a `--format text` user now gets the JSON envelope on stderr, whose
  `message`/`hint` fields remain clearly readable). Do not preserve the `✗` path or
  gate it behind a flag.
- **Warnings: new JSON diagnostic line.** Introduce a structured, single-line JSON
  diagnostic envelope for non-fatal advisories, distinct from the error envelope by
  a `level` field. One diagnostic per line (so stderr stays newline-delimited JSON
  and multiple warnings are individually parseable).

## Design

### Diagnostic envelope (`src/internal/output`)

Add a diagnostic envelope and emitter alongside the existing error types:

```go
// Diagnostic is a non-fatal, machine-readable advisory written to stderr. It
// shares the envelope shape of an error but carries level:"warning" and never
// changes the exit code. One Diagnostic is emitted per line.
type diagnosticEnvelope struct {
    SchemaVersion int    `json:"schema_version"`
    Level         string `json:"level"`            // "warning"
    Code          string `json:"code"`
    Message       string `json:"message"`
    Hint          string `json:"hint,omitempty"`
    Details       any    `json:"details,omitempty"`
}

// EmitWarning writes one JSON diagnostic line to w (stderr). Best-effort, like
// EmitError: a failure to write the diagnostic sink is discarded.
func EmitWarning(w io.Writer, code, message, hint string, details any)
```

Reuse `SchemaVersion` (`output/types.go`, currently `1`). Keep the error envelope
(`ok:false`) as-is for errors; diagnostics use `level` rather than `ok` so the two
are unambiguous to a consumer.

### `EmitError` signature change

Change `EmitError(w io.Writer, format string, err error)` to `EmitError(w
io.Writer, err error)` (always JSON). Update the sole caller in `main.go`. Remove
the now-dead `text` branch in `render()` and the `format` parameter threading. The
`render`/`detailFor`/`classifyUsageError` logic is otherwise unchanged.

### Warning sink threaded to analyses (for `cod-btfg`)

Analyses run via `Descriptor.Run(mods, opts) (any, error)` and have no writer.
Give `analysis.Opts` an optional warning sink so an analysis can raise an advisory
without importing `output` or knowing about stderr:

```go
// In internal/analysis (Opts): a nil Warn means "discard" (the zero value), so
// analyses can call it unconditionally.
type WarnFunc func(code, message, hint string, details any)
// Opts.Warn WarnFunc  // set by the action layer; nil = no-op
```

The action layer (`actionFor` in `src/cmd/codelens/commands.go`) sets
`opts.Warn` to a closure that calls `output.EmitWarning(cmd.Root().ErrWriter,
...)`. Keep `analysis` free of an `output` dependency: the closure lives in the
`main`/action layer and adapts to `output`. `analysisOpts()` builds `Opts`, so add
the sink there (or in `actionFor` right after). A nil-safe helper on `Opts`
(`func (o Opts) warn(...) { if o.Warn != nil { o.Warn(...) } }`) keeps call sites
clean.

This ticket only establishes the facility (and a no-op default); `cod-btfg` is the
first consumer. Do not add a warning caller here beyond what a test needs.

## TDD plan (/tdd)

Behavior through public functions and the CLI, not internals.

1. `TestEmitError_AlwaysJSON`: call `EmitError` — assert a valid JSON error
   envelope is written (parse it; check `ok:false`, `code`, `message`). Table over
   several errors (coded, usage-classified, plain). Replaces the text-format
   assertions.
2. Update `src/internal/output/errors_test.go`: remove/replace the `✗` text-render
   cases; assert JSON for what were previously the `text` cases.
3. `TestEmitWarning_Shape`: `EmitWarning(buf, "code","msg","hint", details)` writes
   exactly one line of JSON with `schema_version`, `level:"warning"`, `code`,
   `message`, `hint`, `details`; a second call appends a second line (NDJSON).
4. `TestEmitWarning_OmitsEmptyHint`: empty hint/details are omitted.
5. CLI-level: a small test that a forced error under `--format text` now yields the
   JSON envelope on stderr (pins the always-JSON decision end to end).
6. `analysis.Opts.warn` nil-safety: calling the helper with a nil `Warn` is a
   no-op (unit test in `internal/analysis`).

One test -> one implementation step. Land the `EmitError` change and its test
first (it touches existing tests), then add `EmitWarning`, then the `Opts` sink.

## Files touched

```text
src/internal/output/errors.go          drop format branch; EmitError(w, err); add EmitWarning + diagnosticEnvelope
src/internal/output/errors_test.go     JSON-always error cases; EmitWarning cases
src/cmd/codelens/main.go               call EmitError(stderr, err); keep --debug slog
src/cmd/codelens/commands.go           set opts.Warn closure -> output.EmitWarning(stderr, ...)
src/internal/analysis/analysis.go      add WarnFunc + Opts.Warn + nil-safe helper (confirm Opts location)
src/internal/analysis/*_test.go        Opts.warn nil-safety
docs/skills/codelens/references/operating.md   note errors are always JSON on stderr; document the warning line + schema
docs/cli-design.md                     update the error/exit-code section (§7) for always-JSON + diagnostics
```

Confirm the exact file that defines `analysis.Opts` (grep `type Opts struct`)
before editing; it may be `analysis.go` or a dedicated file.

## Acceptance criteria

- Every error `codelens` reports on stderr is the JSON error envelope, for all
  `--format` values including `text` and `table`; there is no `✗` text error path.
- `EmitError`'s signature no longer takes `format`; the only caller is updated.
- `output.EmitWarning` emits one JSON diagnostic line per call with
  `schema_version`, `level:"warning"`, `code`, `message`, and optional
  `hint`/`details`; multiple warnings are valid NDJSON.
- `analysis.Opts` carries an optional, nil-safe warning sink; the action layer
  wires it to `output.EmitWarning` on stderr; `internal/analysis` does not import
  `internal/output`.
- Exit codes are unchanged (`output.ExitCodeFor`); warnings never alter the exit
  code, and stdout stays results-only.
- `make build` green; operating.md and cli-design.md document the always-JSON
  errors and the warning line; Markdown passes markdownlint per project standard.

## References

- `src/internal/output/errors.go` (EmitError, render, detailFor, ExitCodeFor)
- `src/internal/output/types.go` (`SchemaVersion`)
- `src/cmd/codelens/main.go` (error rendering + `--debug` slog)
- `src/cmd/codelens/commands.go` (`actionFor`, `analysisOpts`)
- `docs/cli-design.md` §7 (errors and exit codes),
  `docs/skills/codelens/references/operating.md` (errors section)
- Downstream consumer: `cod-btfg` (grouped-coupling warning)
- Skills: `/golang` (error values, one-handle-per-error, no `output` import from
  `analysis`), `/tdd`, `/llm-coding`
