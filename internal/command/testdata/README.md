# codelens golden test fixtures

`authors.log` is the shared input log that drives the golden end-to-end tests in
`golden_test.go`. The goldens freeze the output/CLI spine (the JSON envelope,
`--fields`, `--rows`, `schema`, the usage and data errors, the renamed I/O error
codes, and the coupling warning) as reviewable diffs.

## Origin and license

`authors.log` is derived from code-maat's
`test/code_maat/end_to_end/simple_git2.txt`: a multi-commit git2 log with
repeated entities, multiple authors, and a binary numstat (`-`/`-`). codelens is
licensed GPL-3.0 to match
[code-maat](https://github.com/adamtornhill/code-maat) (also GPL-3.0), so its
test corpus may be reused directly.

## Expected `authors` result

Four entities, ordered by distinct-author count then revision count descending,
with entity name breaking ties ascending:

| entity                             | n_authors | n_revs |
| ---------------------------------- | --------- | ------ |
| src/code_maat/parsers/git2.clj     | 2         | 2      |
| src/code_maat/parsers/git.clj      | 1         | 2      |
| doc/architecture.png               | 1         | 1      |
| src/code_maat/analysis/authors.clj | 1         | 1      |

## Golden naming scheme

Each scenario in the `goldenCases` table (`golden_test.go`) owns three golden
files, named `<scenario>.<artifact>`:

- `<scenario>.out` - the exact stdout bytes.
- `<scenario>.err` - the exact stderr bytes. An empty stderr is an EMPTY file,
  never an absent one; a missing golden fails the run rather than passing as an
  empty assertion.
- `<scenario>.exit` - the process exit code as decimal digits plus a trailing
  newline.

Three files per scenario keep diffs readable: a change to one stream touches one
file, so `git diff` shows at a glance which stream moved. The scenario names are
the `name` field of each `goldenCase`; for example `authors_json`,
`usage_unknown_flag`, `log_open_failed`, `coupling_warning`, and `schema_list`.

## Normalization tokens

Volatile values are replaced with stable tokens before comparison AND before a
golden is written under `-update`, so a golden can never hold a volatile value in
the first place. Unnormalized volatile values are the harness's main failure mode
(flaky goldens), so the discipline is load-bearing.

- `<TMPDIR>` stands for a per-run `t.TempDir()` path. Scenarios that must name a
  filesystem target (a missing `--log` or `--group` file) write `<TMPDIR>` in
  their args; the harness swaps it for the real temp dir before invoking the
  command and swaps it back out of both captured streams afterward. It appears in
  the `log_open_failed` and `input_file_open_failed` goldens, in both the
  `message` and the `details` path.

The `--version` output (which varies with the build) is deliberately not
goldened here; `main_test.go` asserts it directly against `version.Current()`.

## Regenerating goldens

```sh
go test ./internal/command/ -run TestGolden -update
```

Regeneration rewrites all three artifacts for every scenario. Review the diff by
hand before committing: these goldens are the frozen contract for the output
surface and the release note's evidence, not a throwaway. A surprising diff means
the code or an earlier change moved the surface; investigate it, do not paper over
it with `-update`.
