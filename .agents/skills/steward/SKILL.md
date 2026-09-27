---
name: steward
description: Drive a go-nodeset pull request to a mergeable state. Covers the local checks to run before every push, updating the branch by rebasing onto main, and triaging the CI jobs. Use when opening, updating, or watching a PR in this repository.
---

<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Steward a go-nodeset PR

## Base branch

- Every PR targets `main`. One logical change per PR. Agent branches are
  named `claude/<topic>`.

## Before every push

1. `make lint` reports 0 issues.
2. `make test` and `make floor` pass: the tests under the race detector, and
   vet and tests with the Go release `go.mod` names.
3. `make cover` passes: every file at 100%. Install the tool with
   `go install github.com/vladopajic/go-test-coverage/v2@latest`.
4. `make tidy` leaves `go.mod` unchanged and reports no requirements.
5. `make reuse` passes (`pip install reuse`), and `make lint-docs` passes if
   Markdown changed.
6. If `.github/` changed: `actionlint`, `shellcheck .github/scripts/*.sh` and
   `make test-release`.
7. Re-read the diff: both SPDX lines on new files, doc comments on exported
   identifiers, tests for new behaviour, a section in `doc/decisions.md`
   for a decision, `README.md`, `AGENTS.md` and the documents in `doc/` in
   step with the change, and Conventional Commit messages with bullet
   bodies.

## Updating the branch

- Rebase onto `main`, never merge `main` into the branch:
  `git fetch origin main && git rebase origin/main`, resolve conflicts, rerun
  the checks above, then `git push --force-with-lease`.

## CI (`.github/workflows/ci.yml`)

| Job | Runs |
|-----|------|
| Test (Go floor), Test (Go current) | `go vet` and `go test -race` with the newest patch of the release line in `go.mod`, and in `mise.toml` |
| Coverage | go-test-coverage against `.testcoverage.yml` |
| Lint | `make tidy` (no requirements, `go.mod` tidy), golangci-lint |
| Markdown | `make lint-docs` |
| REUSE | `reuse lint` |
| Vulnerabilities | govulncheck on the current line |
| Release tag verification | `.github/scripts/verify-release-tag_test.sh` |

- Every failure is this PR's to root-cause and fix. Never skip or disable a
  test, or lower a coverage threshold.
- Vulnerabilities fails for a new advisory against the standard library
  without any change in the PR. Say so on the PR; the fix is a Go patch
  release, which the job picks up when it is out.

## GitHub etiquette

- Ask the maintainer before posting any comment or review reply, and never
  @-mention anyone.
- Never merge PRs, push to `main`, or create tags or releases.
