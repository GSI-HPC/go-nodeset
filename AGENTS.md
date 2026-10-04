<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# AGENTS.md

go-nodeset is the Go module `github.com/GSI-HPC/go-nodeset`: one package,
`nodeset`, at the repository root, which parses, folds and expands
ClusterShell node sets. API reference:
<https://pkg.go.dev/github.com/GSI-HPC/go-nodeset>. `doc/README.md` maps the
rest of the documentation. Personal, uncommitted instructions belong in
`AGENTS.local.md` or `CLAUDE.local.md` (both gitignored).

## Status

The engine, moved here with its history from the repository where it was
written, which `README.md` names. Stable from v1.0.0: a minor or patch
release breaks no API, expression or output (see Compatibility).

## Layout

- `doc.go`: the package comment. The package stays at the root; a
  subdirectory `nodeset/` would stutter in the import path.
- `nodeset.go`: `NodeSet`, `Parse`, `ParseWith`, the options and the set
  operations.
- `parse.go`: the expression parser, its operators and the budget that
  caps what an expression may name.
- `group.go`: the hosts of a set that share a pattern, kept as a product
  of ranges or listed one by one.
- `rangeset.go`: bracketed ranges, their steps and padding.
- `fold.go`: folding into `String()` and `Hostlist()`, the fold a set keeps,
  and the order `Expand` lists hosts in; `foldnd.go`: folding names with
  several numbers as ClusterShell does.
- `resolver.go`: `Resolver`, the optional `Lister` and `BatchResolver`, and
  `MapResolver`.
- `*_test.go`: unit tests (`nodeset_test.go`), the ClusterShell corpus
  (`clustershell_test.go`), `FuzzParseFold` and `FuzzParseBatch`
  (`fuzz_test.go`), the nD fold against a transcription of ClusterShell's
  (`foldnd_test.go`), the examples (`example_test.go`, and
  `example_batch_test.go`, which pkg.go.dev shows whole), and
  `export_test.go`, which lets tests lower the limits.
- `testdata/`: the ClusterShell corpus, `clustershell.txt`;
  `clustershell.py`, which records ClusterShell's answers;
  `clustershell_oracle.py`, which answers `TestClusterShellOracle`
  (`clustershell_oracle_test.go`); and `requirements.txt`, the ClusterShell
  version both use. Failing fuzz inputs go under `testdata/fuzz/`.
- `doc/`: `README.md` maps the documentation, `language.md` is the
  reference of the node set language, `testing.md` says how the package is
  tested, `decisions.md` records what was decided and why, and
  `release.md` says how a release is cut.
- `.github/workflows/ci.yml`: tests on both Go lines, coverage, fuzzing, the
  comparison with ClusterShell, lint, Markdown, REUSE, govulncheck and the
  tag verification test.
  `release.yml`: verifies a pushed `v*` tag and publishes the release.
- `.github/actions/setup-go`: Go at the newest patch of the `floor` (go.mod)
  or `current` (mise.toml) release line.
- `.agents/skills/`: skills for coding agents; `.claude/skills` links there.

## Commands

```bash
mise install          # Go and golangci-lint at the versions CI uses
make lint             # golangci-lint v2 (.golangci.yml, gofmt + goimports)
make test             # go test -race ./...
make floor            # vet and test with the go line of go.mod
make cover            # go-test-coverage: every file at 100% (.testcoverage.yml)
make fuzz             # FuzzParseFold and FuzzParseBatch for 90 s each, as CI runs them (FUZZTIME=10m for longer)
make clustershell     # compare with ClusterShell itself, record the corpus again (needs python3; CLUSTERSHELL_CASES=)
make tidy             # go mod tidy, and fail if go.mod requires anything
make vuln             # govulncheck
make reuse            # reuse lint (pip install reuse)
make lint-docs        # markdownlint-cli2 (needs npx)
make test-release     # the tag verification script against scratch tags
```

go-test-coverage: `go install github.com/vladopajic/go-test-coverage/v2@latest`.

## Code conventions

- Every file carries the two SPDX lines this one starts with, the copyright
  holder and the licence, in its own comment style (decision 1); a `SKILL.md`
  carries them after its front matter. A file that cannot hold a comment
  would be annotated in a `REUSE.toml`.
- The standard library only, in tests too: no testify, no go-cmp (decision 3).
  Table-driven tests with `t.Errorf`/`t.Fatalf`, and `testing.TB` in
  helpers.
- Every file stays at 100% statement coverage. A branch no test can reach is
  deleted, not excluded.
- Every exported identifier has a doc comment. Usage is shown in `Example`
  functions, which pkg.go.dev renders.
- Wrap errors with `fmt.Errorf("context: %w", err)`. An exported error value
  or type is API.
- The folded output of `String()` and `Hostlist()` is an interface, which
  programs parse and compare: changing it is a breaking change.
- Fuzz targets sit next to the code, and a failing input found by CI is
  committed under `testdata/fuzz/`.
- Comments and documentation are plain British English.
- Never commit a `go.work` or a `replace` directive. To try a change in a
  program that imports the module, put an uncommitted `go.work` in a parent
  directory.

## Compatibility

- The `go` line is the oldest Go release the Go project supports, as a `.0`
  release, with no `toolchain` line (decision 2). Raising it is a commit of
  its own; the `bump-go` skill covers it.
- Semantic versions from v1.0.0 (decision 10). A fix is a patch release;
  new API, a new form of expression or a higher `go` line is a minor one.
  A breaking change to the API, to what an expression names, or to the
  output of `String`, `Hostlist` or `Expand` needs a major version with
  the module path `/v2`, and a decision of its own.
- Nothing in the tree names a version (decision 4). A pushed tag is never
  moved or deleted; a broken release is retracted in `go.mod`.

## Branches, releases and commits

- PRs target `main`. Update a feature branch by rebasing it onto `main`
  (never merge `main` in), then `git push --force-with-lease`.
- Releases are signed `vX.Y.Z` tags, created by the maintainer
  (`doc/release.md`). Never push to `main`, create tags or releases, or
  merge PRs.
- Ask the maintainer before commenting on issues or PRs, and never
  @-mention anyone.
- Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`,
  `build:`, `ci:`), one logical change per commit, bullet-list bodies that
  say what changed and why, without development narrative.
- Commits by Claude Code are authored as `Claude <noreply@anthropic.com>`
  and carry a `Co-Authored-By: Claude …` trailer. Keep both; the README AI
  disclosure relies on them.
- A change that takes a decision adds it to `doc/decisions.md`. A change
  of behaviour updates the doc comments, and the document in `doc/` that
  describes it, in the same PR.

## Skills

- `.agents/skills/steward`: the PR and CI routine.
- `.agents/skills/bump-go`: moving the Go release lines after a Go release.
