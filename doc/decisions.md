<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Decisions

One section per decision, in the order they were taken. Each says what the
situation was, what was decided, and what that costs. A decision is never
edited: a later one supersedes it, and the earlier one's status names it.

| | Decision | Status |
| --- | --- | --- |
| [1](#1-apache-20-and-gsi-holds-the-copyright) | Apache-2.0, and GSI holds the copyright | accepted |
| [2](#2-the-go-line-is-the-oldest-go-release-still-supported) | The go line is the oldest Go release still supported | accepted |
| [3](#3-the-module-requires-nothing) | The module requires nothing | accepted |
| [4](#4-a-release-is-a-signed-tag) | A release is a signed tag | accepted |
| [5](#5-an-engine-of-its-own-rather-than-an-existing-go-library) | An engine of its own rather than an existing Go library | accepted |

## 1. Apache-2.0, and GSI holds the copyright

Status: accepted

### Context

The node set engine was written for [clusterctl](https://github.com/GSI-HPC/clusterctl),
which GSI licenses under LGPL-3.0-or-later as its copyright holder. As a
module of its own the engine is meant for any program that handles node
sets, and for such a program the Lesser GPL is a burden in Go, where every
program links its packages statically: a proprietary program that imports
the engine has to let its users relink it (section 4 of the LGPL).
Libraries in the Go ecosystem are permissive; the Go project's own
modules, `golang.org/x/...`, are BSD-3-Clause.

### Decision

The module is licensed under the Apache License, Version 2.0, and GSI
Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> holds the
copyright.

- `LICENSE` holds the licence text verbatim, so that GitHub and pkg.go.dev
  recognise it. pkg.go.dev shows the documentation only of a module whose
  licence it recognises as redistributable.
- `LICENSES/Apache-2.0.txt` holds it again for the REUSE specification.
- Every file carries `SPDX-FileCopyrightText` and `SPDX-License-Identifier`
  in its own comment style, and `reuse lint` checks this in CI.
- The year is 2026, when the work was first written, and it is not moved
  forward.
- There is no `NOTICE` file.

### Why Apache-2.0

- It grants a licence to every patent a contributor holds that the
  contribution uses, and ends that licence for anyone who sues over one.
  BSD-3-Clause, Go's own licence, says nothing about patents; the Go project
  adds a grant of Google's in a separate `PATENTS` file. MIT is silent
  too.
- It is compatible with LGPL-3.0 and GPL-3.0, so programs under either
  licence can import the module.

### Moving code in

GSI holds the copyright of the code that moves here, so it can license this
copy under Apache-2.0. The commit that moves a file changes its
`SPDX-License-Identifier` line. The code keeps its LGPL-3.0-or-later
licence in the history of the repository it comes from.

### Costs

- Apache-2.0 is not compatible with GPL-2.0-only, so a program under that
  licence cannot import the module.
- A redistributor keeps the copyright and licence notices of every file.
- A contributor from outside GSI keeps the copyright of their contribution
  and licenses it under Apache-2.0 (section 5); a file they change then
  names them in an `SPDX-FileCopyrightText` line of their own.

## 2. The go line is the oldest Go release still supported

Status: accepted

### Context

Since Go 1.21 the `go` line in `go.mod` is a requirement, not a hint.
An older toolchain refuses the module, or downloads a newer one, and every
module that requires this one inherits at least this `go` line when it
runs `go mod tidy`. The line is therefore the oldest Go that every
importer has to use.

The Go project ships a release in February and in August. It supports the
two newest, and only those receive security fixes. In September 2026 those
are Go 1.27 and Go 1.26.

The engine's code compiles with Go 1.24 already, but Go 1.24 has had no
security fixes since February 2026.

### Decision

- `go.mod` names the oldest Go release that the Go project still supports,
  as the `golang.org/x` modules do: `go 1.26.0` today. It names a `.0`
  release, never a patch release, and `go.mod` has no `toolchain` line.
- When a new Go release ships, the line moves up to the release before it,
  in a commit of its own, and the next release of the module is a minor
  version. Raising it further takes a record of its own.
- `mise.toml` names the newest Go release line. Contributors and CI use its
  newest patch.
- CI tests both ends: the newest patch of the release `go.mod` names, and
  that of the one `mise.toml` names.

### Costs

- A program built with a Go release that is no longer supported cannot take
  a new version of the module. It keeps the version it has.
- The line is moved twice a year, by hand, since Dependabot does not; the
  `bump-go` skill describes how.

## 3. The module requires nothing

Status: accepted

### Context

The node set engine is a small library that command-line tools embed, and
each of them takes on every requirement it has: minimal version selection
raises their versions of shared modules to the highest any requirement
asks for. The engine uses the standard library alone.

### Decision

`go.mod` has no `require` line, and there is no `go.sum`. The tests
use the standard library alone too: no testify, no go-cmp.

- `make tidy`, which CI runs, fails when `go list -m all` names any
  module but this one.
- golangci-lint's depguard allows imports of the standard library and of
  this module only.
- Dependabot has no Go modules to update.

### Costs

Whatever the standard library lacks is written here, test helpers
included, or done without.

## 4. A release is a signed tag

Status: accepted

### Context

A version of a Go module is a tag. The module proxy serves any tag of a
public repository as soon as someone asks for it, and the checksum database
records its content for good: deleting or moving the tag afterwards does not
unpublish it.

### Decision

- Nothing in the tree names a version: no `VERSION` file, no constant, no
  `CHANGELOG.md`, and `CITATION.cff` has no `version`.
- The maintainer releases by pushing an annotated tag `vX.Y.Z`, signed with
  an SSH or an OpenPGP key. The body of the tag message, everything after
  its first line, is the release notes.
- The Release workflow refuses a tag that no key in the
  `RELEASE_ALLOWED_SIGNERS` (SSH) or `RELEASE_ALLOWED_PGP_KEYS` (OpenPGP)
  repository variable signed under the name it was pushed as, and a tag the
  go command would not accept as a version of this module. It tests the
  tagged commit on both Go lines, publishes the GitHub release with the tag
  body as its notes, and fetches the version through proxy.golang.org, so
  that pkg.go.dev lists it.
- A tag is never moved or deleted. A broken release is withdrawn with a
  `retract` directive in `go.mod`, which ships in the next release.
- The module stays at v0 while its API settles, and a v0 minor release may
  break it; the release notes say how. v1.0.0 follows once the API has held
  still through releases of the programs that import the module.

`doc/release.md` says how to cut a release and how to set up the
verification.

### Costs

- The workflow cannot stop the module proxy from serving a tag it refused.
  Its failure is the alarm, and retraction is the remedy; a tag ruleset that
  lets only the maintainers create `v*` tags is what keeps others from
  pushing one.
- The signing key becomes part of the release process. Two formats are
  accepted, so that a maintainer signs with the key they already use; the
  verification and its tests cover both.
- An OpenPGP key is trusted as the variable holds it: an expired key stops
  verifying on its own, a revoked one only once the variable holds its
  revocation.

## 5. An engine of its own rather than an existing Go library

Status: accepted

### Context

The engine was written for [clusterctl](https://github.com/GSI-HPC/clusterctl),
whose [ADR 0002](https://github.com/GSI-HPC/clusterctl/blob/v0.4.0/doc/adr/0002-own-nodeset-engine.md)
decided to write it rather than depend on one. That record gave as its
reason that Go had no node set implementation. That was wrong.

Before the engine moved here, in September 2026, a review of prior art
looked for Go packages that parse node sets or host lists, ran them over the
ClusterShell corpus in `testdata/clustershell.txt`, which then held 45
expressions, and read what each requires:

| Implementation | Expressions answered as the corpus records |
| --- | --- |
| This engine | 45 of 45 |
| The best other Go library | 30 of 45 |
| The next four | 28, 23, 23 and 22 of 45 |

What stands in the way of the others:

- grendel and iskylite are GPL-3.0, and live inside large application
  modules.
- DAOS's `lib/hostlist` reads pdsh's syntax rather than ClusterShell's, has
  44 requirements, and has pseudo-versions only.
- cc-lib/v2 has 35 requirements.
- puttsk and the rest are stubs.

No library that came close is usable as a dependency: the nearest are GPL,
or pull in a large tree of requirements. CEA's sshproxy, a Go program from
the authors of ClusterShell, loads a node set library written in Rust
through purego rather than use a Go one.

### Decision

- The module keeps an engine of its own, and requires nothing
  (decision 3).
- Its compatibility with ClusterShell is measured rather than claimed. The
  corpus records ClusterShell's answer for every expression, and a test
  holds the package to it. Every place where the package decides otherwise
  is marked in the corpus and listed in `doc/language.md`, and a test checks
  that the two lists agree.
- This corrects the prior-art claim of clusterctl's ADR 0002. That record
  still holds in clusterctl's history as it was written.

### Costs

- Every bug is this module's to find and fix: there is no upstream to report
  one to, and no other users finding them first.
- ClusterShell moves on without it. The corpus records ClusterShell 1.10.1,
  and `testdata/clustershell.py` has to be run again to notice where a
  newer release changed its answers.
- A second implementation of the language comes with divergences of its own,
  which a user of both tools meets as a set that the two read differently.
  `doc/language.md` lists them.
- One maintainer releases one more module.
