---
id: cod-174y
status: closed
deps: []
links: [cod-gwio]
created: 2026-07-24T20:37:12Z
type: task
priority: 1
assignee: Andre Silva
---

# Migrate Go module from src/ to repo root

## Goal

The codelens Go module currently lives at `src/go.mod` but declares the module
path `github.com/andreswebs/codelens`. The Go toolchain maps that module path to
a `go.mod` at the repository root, so as published the module is unreachable
through the module proxy: `go install github.com/andreswebs/codelens@latest`
fails, and pkg.go.dev, govulncheck, and dependabot cannot resolve it.

This plan moves the module contents from `src/` up to the repository root so the
declared module path matches its on-disk location. Import paths do not change
(the module path already omits `src`), so no `.go` file needs editing. codelens
is a CLI only (`cmd/codelens`); there is no library surface and no Docker or Nix
packaging to adjust.

This is a planning and implementation guide. Branching, commits, and tags are
handled by the repository owner. Do not create git commits or tags.

## Current layout (verified)

The module root under `src/` contains exactly five entries:

```text
src/
  .golangci.yml
  cmd/            (cmd/codelens: CLI entrypoint and its tests + testdata)
  go.mod
  go.sum
  internal/       (analysis, gitlog, model, output, pipeline, terr, transform, version)
```

Target layout after migration:

```text
codelens/
  go.mod
  go.sum
  .golangci.yml
  cmd/codelens/...
  internal/...
  Makefile, README.md, AGENTS.md, docs/, bin/, dist/, .github/, ...  (unchanged position)
```

Toolchain confirmed: `go version go1.26.5`. `go.mod` declares `go 1.26.5` and
requires `github.com/urfave/cli/v3 v3.10.1` and
`github.com/bmatcuk/doublestar/v4 v4.10.0`. Existing tags: v0.0.1, v0.0.2,
v0.0.3 (none resolvable via the proxy today).

## Step 1: move the module contents to the repo root

Run every command from the repository root (the directory that holds `Makefile`,
`README.md`, and `src/`). Use `git mv` so history is preserved.

```sh
git mv src/go.mod src/go.sum src/.golangci.yml src/cmd src/internal .
rmdir src
```

After this, `git status` should show the five entries relocated and `src/`
removed. Confirm nothing is left behind:

```sh
test ! -e src && echo "src removed"
ls go.mod go.sum .golangci.yml cmd internal
```

Note: `.golangci.yml` is a dotfile; the explicit `git mv src/.golangci.yml .`
above handles it (a glob would miss it). There are no other hidden files in
`src/`.

## Step 2: Makefile

File: `Makefile`. After the move, `$(CURDIR)` is the module root, so the
`SRC_DIR` variable and every `cd $(SRC_DIR) &&` prefix are redundant. Recommended
resolution of the fleet draft's open question 1 for this repo: remove `SRC_DIR`
and drop the `cd` prefixes. The Makefile already sits at the module root, so this
is both correct and the cleaner end state; the remaining variables (`BIN_DIR`,
`DIST_DIR`, `CMD_DIR`, `LOCAL_BIN`) are unaffected.

Edit 2a, line 2, delete the `SRC_DIR` line:

Before:

```make
APP_NAME    := codelens
SRC_DIR     := $(CURDIR)/src
BIN_DIR     := $(CURDIR)/bin
```

After:

```make
APP_NAME    := codelens
BIN_DIR     := $(CURDIR)/bin
```

Edit 2b, line 29 (`build-local` recipe):

Before:

```make
 cd $(SRC_DIR) && CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(LOCAL_BIN) $(CMD_DIR)
```

After:

```make
 CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(LOCAL_BIN) $(CMD_DIR)
```

Edit 2c, line 33 (inside the `build-target` define):

Before:

```make
 cd $(SRC_DIR) && CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME)-$(1)-$(2)$(3) $(CMD_DIR)
```

After:

```make
 CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME)-$(1)-$(2)$(3) $(CMD_DIR)
```

Edit 2d, lines 69, 72, 75, 78, 81, 84, 87 (the `run`, `test`, `test-race`,
`vet`, `fmt`, `fmt-check`, `lint` recipes). Drop the `cd $(SRC_DIR) &&` prefix
from each. Exact after state:

```make
run: ## Run locally with build flags
 go run $(BUILDFLAGS) -ldflags="$(LDFLAGS)" $(CMD_DIR)

test: ## Run tests
 go test ./...

test-race: ## Run tests with race detector
 go test -race ./...

vet: ## Run go vet
 go vet ./...

fmt: ## Format Go code
 gofmt -w .

fmt-check: ## Check Go formatting
 @test -z "$$(gofmt -l .)" || (echo "files not formatted:"; gofmt -l .; exit 1)

lint: ## Run linter
 golangci-lint run ./...
```

`CMD_DIR := ./cmd/codelens` (line 5) is already relative to the module root and
needs no change. `LDFLAGS` (line 7) references
`github.com/andreswebs/codelens/internal/version.Override`; the module path is
unchanged, so this stays exactly as is.

Minimal-diff alternative (not recommended, listed for completeness): keep every
`cd $(SRC_DIR) &&` prefix and change only line 2 to `SRC_DIR := $(CURDIR)`. This
produces a one-line diff but leaves a pointless indirection in place.

## Step 3: GitHub Actions workflows

### `.github/workflows/ci.yml`

Edit lines 23-24 in the "Set up Go" step:

Before:

```yaml
with:
  go-version-file: src/go.mod
  cache-dependency-path: src/go.sum
```

After:

```yaml
with:
  go-version-file: go.mod
  cache-dependency-path: go.sum
```

No other `src/` references and no comments mentioning `src` exist in this file.

### `.github/workflows/release.yml`

Edit 3a, lines 25-26 (the "setup-go" step), same change as CI:

Before:

```yaml
with:
  go-version-file: src/go.mod
  cache-dependency-path: src/go.sum
```

After:

```yaml
with:
  go-version-file: go.mod
  cache-dependency-path: go.sum
```

Edit 3b, line 52 (the "Generate SBOM" step, `anchore/sbom-action`). Currently:

```yaml
with:
  path: src
  format: spdx-json
  output-file: dist/codelens-${{ github.ref_name }}.spdx.json
  upload-artifact: false
```

Today `path: src` scans only the Go module source. After migration the module
lives at the repo root alongside `docs/`, `bin/`, and `dist/`. Critically, the
SBOM step runs after `make dist` (line 35), so at that point `dist/` holds the
built `.tar.gz`/`.zip` archives and `bin/` holds the cross-compiled binaries.
Pointing Syft at a bare `.` would make it catalog those binaries and inflate or
corrupt the SBOM. Resolution of the fleet draft's open question 2 for this repo:
scan the repo root but exclude build artifacts and non-Go directories via a Syft
config file.

Change line 52 to:

```yaml
path: .
```

And create a new file `.syft.yaml` at the repo root:

```yaml
exclude:
  - ./bin
  - ./dist
  - ./docs
  - ./.git
  - ./.github
  - ./.venv
  - ./.ruff_cache
  - ./.tickets
  - ./.local
```

Syft reads `.syft.yaml` from the working directory automatically; anchore/sbom-action
runs from the repo root, so no extra wiring is needed. This keeps the SBOM scoped
to the Go module (go.mod, go.sum, and the source tree) while still resolving the
module at its new location. `SYFT_SOURCE_NAME`/`SYFT_SOURCE_VERSION` (lines 49-50)
already pin the SBOM's root package identity and stay unchanged.

The `.github/actions/gh-release/action.yml` and its `create-release.bash` script
contain no `src/` references (the `codelens-*` archive names come from `make dist`
and are unaffected). No edits there.

## Step 4: dependabot

File: `.github/dependabot.yml`, line 10. The gomod ecosystem points at `/src`.

Before:

```yaml
- package-ecosystem: gomod
  directory: /src
  schedule:
    interval: weekly
```

After:

```yaml
- package-ecosystem: gomod
  directory: /
  schedule:
    interval: weekly
```

The `github-actions` entry already uses `directory: /` and is unchanged.

## Step 5: fleet workspace (go.work)

File: `../go.work` at the fleet root (the parent `go-projects` workspace). Change
the codelens `use` entry:

Before:

```text
use (
 ./codelens/src
 ./dn-tool/src
 ...
```

After:

```text
use (
 ./codelens
 ./dn-tool/src
 ...
```

Then, from the fleet root, run `go work use` to normalize the file:

```sh
go work use ./codelens
```

Leave the other repos' entries untouched; they migrate independently. The fleet
root `AGENTS.md` states that each module lives at `<project>/src/`; that is a
fleet-wide statement and is corrected when the whole fleet has migrated (or
amended incrementally). It is out of scope for the codelens change itself.

## Step 6: sweep for stragglers

Run the sweep from the repo root:

```sh
grep -rn 'src/' --exclude-dir=.git --exclude-dir=.venv --exclude-dir=.ruff_cache --exclude-dir=.tickets .
```

Expected findings and how to treat each:

1. `README.md` (around lines 124, 144-145, 159-160, 171-172, 189-190) and other
   docs contain example entity paths like `src/code_maat/parsers/git2.clj`. These
   are sample analysis output (paths from an analyzed foreign repository), NOT
   references to codelens's own layout. Do NOT change them. Changing them would
   corrupt the documented examples.
2. `docs/specs/learnings.md` (lines 336, 1030, 1058) references old code
   locations such as `src/internal/transform/filter` and
   `src/internal/analysis/coupling.go`. This is a historical learnings log
   describing the state at the time each entry was written. Recommended: leave
   these as-is (they are a dated record). Optionally, if the owner prefers a
   clean tree, rewrite them to drop the `src/` prefix, since the code now lives
   at `internal/...`. Not build-critical either way.
3. Any remaining hit in Makefile, `.github/`, or `dependabot.yml` means an edit
   from steps 2-4 was missed; fix it.

There are no `src/` references in `.editorconfig`, `.gitignore`, `.envrc`,
`.markdownlint.yaml`, `AGENTS.md`, or `CLAUDE.md` (a symlink to `AGENTS.md`).
The Python tooling (`.venv`, `.ruff_cache`, uv via `.envrc` which is just
`layout uv`) does not reference `src/` and needs no change.

## Step 7: verify locally

From the repo root:

```sh
make validate   # fmt-check, vet, lint, test (all now run at the module root)
make build      # validate + compile the local binary into bin/
```

Run the freshly built binary end to end (it reads a git log on stdin and is
read-only):

```sh
./bin/codelens-$(go env GOOS)-$(go env GOARCH) --version
./bin/codelens-$(go env GOOS)-$(go env GOARCH) schema
git -C . log --pretty='%H%n%an%n%ad%n%s' --numstat | ./bin/codelens-$(go env GOOS)-$(go env GOARCH) authors
```

Also confirm the workspace still resolves from the fleet root:

```sh
go -C .. build ./codelens/...
```

## Step 8: external proxy verification (after merge and a new tag)

The owner merges the change and tags a new release (next tag after v0.0.3, for
example v0.0.4). The old tags were never resolvable through the proxy, so nothing
needs retracting. After the release workflow publishes the tag, verify from a
directory OUTSIDE the workspace so the local `go.work` does not mask the proxy:

```sh
cd "$(mktemp -d)"
GOFLAGS=-mod=mod GOPROXY=proxy.golang.org GO111MODULE=on \
  go install github.com/andreswebs/codelens/cmd/codelens@latest
"$(go env GOPATH)/bin/codelens" --version
```

`go install github.com/andreswebs/codelens@latest` (module root form) also works
because the single main package under `cmd/codelens` is the module's command; if
the owner prefers the shorter form, confirm which path Go selects. The explicit
`cmd/codelens` path above is unambiguous.

## Rollback

The change is a pure file move plus build-config edits. To roll back before any
commit, restore the tree with git:

```sh
git checkout -- Makefile .github/workflows/ci.yml .github/workflows/release.yml .github/dependabot.yml
git mv go.mod go.sum .golangci.yml cmd internal src   # if the move was staged; recreate src first
```

Simpler: since nothing is committed until the owner does so, `git reset --hard`
to the pre-change state (or `git stash`) reverts the entire migration. The
`../go.work` edit is reverted the same way at the fleet root. No published
artifacts are affected until a new tag is pushed, so rollback before tagging has
no external consequences.

## Files changed (summary)

- `src/go.mod`, `src/go.sum`, `src/.golangci.yml`, `src/cmd/`, `src/internal/`
  moved to the repo root; `src/` removed.
- `Makefile`: remove `SRC_DIR`; drop `cd $(SRC_DIR) &&` from lines 29, 33, 69,
  72, 75, 78, 81, 84, 87.
- `.github/workflows/ci.yml`: lines 23-24 setup-go paths.
- `.github/workflows/release.yml`: lines 25-26 setup-go paths; line 52 SBOM
  `path: src` to `path: .`.
- `.syft.yaml`: new file at the repo root (SBOM exclude config).
- `.github/dependabot.yml`: line 10 gomod `directory: /src` to `/`.
- `../go.work` (fleet root): `./codelens/src` to `./codelens`.
- No `.go` file changes. No changes to README example data or (recommended)
  historical learnings-log entries.

## Notes

**2026-07-24T20:48:45Z**

Moved go.mod/go.sum/.golangci.yml/cmd/internal from src/ to repo root via git mv (renames, history preserved); src/ removed. No .go edits (module path already omitted src). Makefile: removed SRC_DIR and dropped 'cd $(SRC_DIR) &&' from all recipes. CI+release setup-go paths and dependabot gomod directory changed /src -> /. release.yml SBOM path src -> . plus new .syft.yaml at repo root excluding bin/dist/docs/etc so Syft (running post-make-dist) does not catalog build artifacts. Step 5 (fleet go.work use entry) N/A: no go.work in this environment. Remaining src/ grep hits are sample analysis entity paths in tests/testdata and historical learnings, left untouched. make build green from root; built binary --version/schema/authors verified end-to-end. Staging/commit left to owner; .syft.yaml is a new untracked file.

**2026-07-24T21:00:43Z**

FLEET DECISION — Makefile (overrides this plan's recommendation).

Standardize on the MINIMAL DIFF, not the "remove SRC_DIR / drop cd prefixes"
resolution written in the plan body above. Concretely:

- Change only line 2 to `SRC_DIR := $(CURDIR)`.
- KEEP every `cd $(SRC_DIR) &&` recipe prefix as-is (they now resolve to the
  repo root).

Ignore Edit 2a and the "delete the SRC_DIR line / drop the cd prefixes" steps.
The rest of the plan stands and already matches the finalized fleet recipe:
dependabot gomod `directory: /src` -> `/`, and SBOM `path: .` plus a root
`.syft.yaml` excluding ./bin/**, ./dist/**, ./docs/**, ./.git/**.

Source: .local/planning/go-module-root-migration.md ("Decisions").

**2026-07-24T21:26:07Z**

Gap closure against the 2026-07-24T21:00:43Z FLEET DECISION (which overrode the plan body's Makefile resolution).

Prior implementation had followed the plan body: Makefile SRC_DIR removed and every 'cd $(SRC_DIR) &&' recipe prefix dropped (the 'clean' end state). The fleet decision reversed this to the MINIMAL DIFF. Corrected:

- Makefile: restored to the pre-migration recipe form and changed only line 2 to 'SRC_DIR := $(CURDIR)'; all 'cd $(SRC_DIR) &&' prefixes kept (build-local, build-target, run, test, test-race, vet, fmt, fmt-check, lint). git diff vs HEAD is now exactly that one line.
- .syft.yaml: aligned to the finalized fleet recipe (exclude ./bin/**, ./dist/**, ./docs/**, ./.git/**). Previously used bare directory names (./bin) which do not recursively exclude directory contents in Syft, plus extra entries (.github/.venv/.ruff_cache/.tickets/.local) not in the fleet standard.

Verified already-correct from prior work: module at repo root (src/ removed), ci.yml/release.yml setup-go use go.mod/go.sum, release.yml SBOM path: ., dependabot gomod directory: /. Step 5 (go.work) N/A: no go.work in this environment.

Verification: 'make build' (validate: fmt-check/vet/lint/test, then compile) green from repo root with the cd-prefixed recipes; built binary --version, schema, and authors (canonical print-log-command log) all exit 0. Staging/commit/tag left to the owner.
