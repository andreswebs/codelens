# gitlog test fixtures

These `*.log` files are git2(+subject) log fixtures used by the golden parser
tests (`parse_golden_test.go`). Each `<name>.log` has a committed
`<name>.golden.json` holding the expected `[]model.Modification`.

## Origin and license

The fixtures model the grammar cases exercised by code-maat's
`test/code_maat/parsers/git2_test.clj` and `test/code_maat/end_to_end/simple_git2.txt`:
a plain commit, a binary numstat, multiple blank-line-separated commits, and
stacked merge/pull-request preludes. codelens is licensed GPL-3.0 to match
[code-maat](https://github.com/adamtornhill/code-maat) (also GPL-3.0), so its
test corpus may be reused directly; these fixtures are derived from that corpus.

## Coverage

| Fixture             | Exercises                                                        |
| ------------------- | ---------------------------------------------------------------- |
| `entry.log`         | Stock 3-field prelude (no subject); message defaults to `-`      |
| `binary.log`        | Extended `--%s` subject; binary file (`-`/`-`) alongside text    |
| `entries.log`       | Two blank-line-separated commits; six records (drift guard)      |
| `pull_requests.log` | Stacked preludes (last wins) plus a following normal commit      |
| `simple_git2.log`   | End-to-end multi-commit log: repeated entities, a binary, subjects |

## Regenerating goldens

```sh
go test ./internal/gitlog/ -run TestParse_Golden -update
```

Review the diff by hand before committing: the JSON goldens are the parser's
regression contract.
