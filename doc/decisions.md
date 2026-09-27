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
