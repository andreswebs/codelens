# codelens end-to-end test fixtures

`authors.log` is a git2(+subject) log fixture that drives the end-to-end golden
tests in `e2e_authors_test.go`. It freezes the output/CLI spine (every format,
`--fields`, `--rows`, and `schema --command`) on the `authors` analysis before
the remaining analyses fan out.

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

| entity                              | n_authors | n_revs |
| ----------------------------------- | --------- | ------ |
| src/code_maat/parsers/git2.clj      | 2         | 2      |
| src/code_maat/parsers/git.clj       | 1         | 2      |
| doc/architecture.png                | 1         | 1      |
| src/code_maat/analysis/authors.clj  | 1         | 1      |

## Goldens

`authors.json`, `authors.ndjson`, `authors.csv`, `authors.table`,
`authors.fields.json` (`--fields rows.entity`), `authors.rows2.json`
(`--rows 2`), and `authors.schema.json` (`schema --command authors`) are the
committed goldens.

## Regenerating goldens

```sh
go test ./cmd/codelens/ -run TestE2E_Authors -update
```

Review the diff by hand before committing: these goldens are the frozen
contract for the output surface, not just the `authors` analysis.
