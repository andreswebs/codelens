---
id: cod-7fb5
status: closed
deps: [cod-vdx3]
links: []
created: 2026-07-14T03:52:50Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-5]
---
# P5-2 cmd: version subcommand + --version

Confirm the version surface: both the "version" subcommand and the --version flag report internal/version.Current().
Edit: src/cmd/codelens/main.go / commands.go; new version_cmd_test.go.
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: cli-design.md 4, requirements 11, docs/research/urfave-cli.reference.md (CLI framework). Depends on P0-1.

## Design

- Root already sets Version (urfave --version). Add a `version` subcommand printing version.Current() to stdout (plain), exit 0.
- Ensure both paths use internal/version.Current() (single source).
TDD:

1. TestVersion_Subcommand: `version` -> stdout contains version.Current().
2. TestVersion_Flag: `--version` -> stdout contains the same value.

## Acceptance Criteria

- version subcommand and --version both report the build version. Cases pass; make validate green.

## Notes

**2026-07-14T13:50:48Z**

Added version subcommand (cmd/codelens/version_cmd.go) printing the plain version.Current() to stdout, exit 0; wired into the root command tree in main.go. --version flag already worked via root Version=version.Current(); both paths now share that single source. Verified on the built binary: 'version' -> '392a49e-dirty', '--version' -> 'codelens version 392a49e-dirty'. Fixed a pre-existing drift: schema.go metaSummaries already advertised a 'version' command that did not exist; updated the versionUsage comment (was 'wired in a later phase'). Tests: version_cmd_test.go (TestVersion_Subcommand, TestVersion_Flag). make build green.
