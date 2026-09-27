---
name: bump-go
description: Move the Go release lines of go-nodeset after a Go release, the go line in go.mod (the oldest supported Go) and the go entry in mise.toml (the newest). Use when a Go major release has shipped (February and August), when CI or govulncheck points at an unsupported Go, or when asked to bump Go.
---

<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Bump the Go release lines

Decision 2 in `doc/decisions.md` sets two numbers:

- `mise.toml` `go = "1.N"`: the newest Go release line. Contributors and
  CI's current jobs use its newest patch.
- `go.mod` `go 1.M.0`: the oldest Go release the Go project still supports,
  which is 1.(N-1) once 1.N has shipped. Always `.0`, never a `toolchain`
  line.

## Steps

1. Find the released lines. go.dev may be unreachable from a sandbox; the
   module proxy lists the toolchains:
   `curl -sS https://proxy.golang.org/golang.org/toolchain/@v/list | grep linux-amd64 | sort -V | tail`.
   Ignore release candidates.
2. If `mise.toml` is behind, move it to the newest line in one commit:
   `build: develop and test with go 1.N`.
3. If `go.mod` is below 1.(N-1): `go mod edit -go=1.(N-1).0`, run
   `make tidy`, and update the Go version `README.md` states. One commit,
   `build: require go 1.(N-1)`, whose body says that 1.(N-2) is out of
   support.
4. Check that the importers are ready: the programs that pkg.go.dev lists
   under *Imported by* must say at least the new line in their `go.mod`. If
   one does not, say so in the PR.
5. Optionally, in a separate `refactor:` commit, adopt what the new floor
   allows: `go fix -diff ./...` shows what the modernizers of `go fix` would
   change, and `go fix ./...` applies it. Review the result.
6. Run the steward checks, `make floor` included.

A higher `go` line ships in a minor release, never in a patch release.
