# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Tool-level error surface in `schema`.** Two additive schema fields let an
  agent enumerate the full error surface at runtime. `schema --command CMD` now
  carries `common_error_codes`, the tool-level baseline any invocation can
  produce (input, global-option, and output-layer failures), reported the same
  on every command; a command's `error_codes` now carries only the codes
  distinctive to it, so the full surface is `error_codes + common_error_codes`.
  `schema` (no `--command`) now carries an `errors` inventory of
  `{code, exit_code, hint}`, derived from the coded-error registry so it cannot
  drift from the sentinels that exist, sorted by code. Both fields are additive,
  so `schema_version` stays at `1`; a consumer that ignores unknown keys is
  unaffected.

### Changed

- **Breaking: error codes are now unique per sentinel.** Three error codes were
  each carried by more than one failure, so a consumer branching on the code
  could not tell unrelated failures apart. Each duplicate is renamed to a
  distinct code. Exit codes are unchanged; `schema_version` stays at `1`, so the
  envelope carries no version signal for this change and this entry is the
  record. Consumers branching on `usage_error` or `io_error` must migrate to the
  new codes below.

  | Old code      | New code                 | Exit | Failure                                        |
  | ------------- | ------------------------ | ---- | ---------------------------------------------- |
  | `parse_error` | `invalid_control_char`   | 65   | disallowed control character in the input      |
  | `usage_error` | `unknown_format`         | 64   | unknown `--format` value                       |
  | `usage_error` | `unknown_schema_command` | 64   | unknown `schema --command` value               |
  | `usage_error` | `invalid_after_date`     | 64   | malformed `print-log-command --after` date     |
  | `io_error`    | `log_open_failed`        | 74   | `--log` path unreadable                        |
  | `io_error`    | `input_file_open_failed` | 74   | `--group` / `--team-map` path unreadable       |

  `parse_error` still marks a structurally invalid log entry.

- **Richer `schema` error inventory.** The `errors` inventory in `schema` (no
  `--command`) now lists four more codes: `unknown_command` (promoted from a
  per-invocation error to a registered sentinel) and the three usage classes
  `unknown_flag`, `invalid_value`, and `missing_required_flag` (previously
  classified from the CLI framework's parse messages without a backing sentinel).
  The inventory is more complete; `schema_version` stays at `1`. Emitted error
  envelopes are unchanged (same codes, hints, and exit codes).

### Removed

- **Breaking: `--debug` removed.** codelens adopts the silent logging posture
  (ADR 0005, local), so it carries no logging infrastructure. The `--debug`
  flag and its `log/slog` diagnostic are gone; no replacement is needed, as
  advisories already flow through the machine-readable warning channel on stderr
  (`output.EmitWarning`). An invocation passing `--debug` now exits 64 with an
  `unknown_flag` error envelope.

### Internal

- **CLI delegate moved behind a `Deps` seam (ADR 0004).** The command tree, the
  root construction, the usage-error classifier, and the CLI-surface sentinels
  moved out of `package main` into a new `internal/command` package. `main` is
  now one line that hands the process streams to
  `command.Run(args []string, deps command.Deps) int`, whose signature names no
  CLI-framework type, so the framework is confined to `internal/command` and a
  no-leak test keeps it there. Args now arrive without the program name
  (`os.Args[1:]`). No user-visible behavior changed.
- **Golden triple harness for the CLI (ADR 0007).** The in-process end-to-end
  tests now compare three artifacts per scenario against golden files (stdout,
  stderr, and the exit code) driven through `command.Run`, replacing the
  stdout-only success harness. Error scenarios (the four usage errors, the three
  `--format` data errors, each renamed I/O and format error code, the
  control-character data error) and the coupling warning (non-empty stderr on a
  zero exit) are now pinned as reviewable golden diffs. Test-only change; no
  user-visible behavior changed.
