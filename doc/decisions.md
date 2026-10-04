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
| [4](#4-a-release-is-a-signed-tag) | A release is a signed tag | accepted, in part superseded by [10](#10-the-first-release-is-v100) |
| [5](#5-an-engine-of-its-own-rather-than-an-existing-go-library) | An engine of its own rather than an existing Go library | accepted |
| [6](#6-clustershell-itself-is-the-reference-in-ci) | ClusterShell itself is the reference in CI | accepted, in part superseded by [7](#7-the-package-prints-what-clustershell-prints) |
| [7](#7-the-package-prints-what-clustershell-prints) | The package prints what ClusterShell prints | accepted, in part superseded by [8](#8-a-set-keeps-its-ranges-and-its-fold) |
| [8](#8-a-set-keeps-its-ranges-and-its-fold) | A set keeps its ranges and its fold | accepted, extended by [9](#9-operators-combine-ranges-where-the-result-is-ranges) |
| [9](#9-operators-combine-ranges-where-the-result-is-ranges) | Operators combine ranges where the result is ranges | accepted |
| [10](#10-the-first-release-is-v100) | The first release is v1.0.0 | accepted |
| [11](#11-mapresolver-evaluates-every-group-of-a-source-on-its-own) | MapResolver evaluates every group of a source on its own | accepted |

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

Status: accepted, in part superseded by
[decision 10](#10-the-first-release-is-v100)

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

## 6. ClusterShell itself is the reference in CI

Status: accepted, in part superseded by
[decision 7](#7-the-package-prints-what-clustershell-prints)

### Context

The package implements ClusterShell's node set language. Until now it was
held to ClusterShell only through the corpus in `testdata/clustershell.txt`:
58 expressions written by hand, with the hosts ClusterShell named for each
recorded once. The corpus test compares the package with those recorded
hosts, not with ClusterShell, and not with ClusterShell's folded output.

Before v0.1.0 the maintainer asked for CI to install ClusterShell and check
that the package gives the same output. A first run over 20,000 generated
expressions found that both name the same hosts for every one of them, and
that the output differs in these ways:

- folding a set whose names hold several numbers, where there is more than
  one right answer and the two choose differently;
- the order in which `Expand` lists such a set;
- autostep: the package never wrote the last two values of a list as a step,
  although its documentation promised a step for any n values;
- a bare `@group` inside a group, an unknown group, and `exe[1-2-3]`.

### Decision

- CI installs the ClusterShell that `testdata/requirements.txt` pins, in a
  virtual environment, and `TestClusterShellOracle` compares the package
  with it on the corpus, on expressions both must reject, and on 20,000
  expressions generated from a fixed seed. The corpus is then recorded again
  with that ClusterShell and must not change.
- Both must reject the same expressions and name the same hosts, and each
  must read what the other prints, the host list included, as the same
  hosts. Where every host name has at most one number, the folded output
  and the order of `Expand` must be ClusterShell's, character for character,
  autostep included.
- Autostep takes the values from left to right as ClusterShell does. That
  fixes the step the package left out; folded output without autostep does
  not change.
- Folding names with several numbers stays the package's own, and so do the
  order of `Expand`, the source of a bare nested group, and an unknown group
  being an error. `doc/language.md` lists these differences, as it lists the
  corpus's.
- The Go test uses the standard library, and it is skipped unless
  `NODESET_CLUSTERSHELL_PYTHON` names a Python with ClusterShell, so `go test`
  needs nothing but Go and decision 3 holds. Python is a tool CI runs, as
  `reuse` is.

### Costs

- The CI job needs PyPI: if ClusterShell cannot be installed, the job fails.
- The pin is raised by hand. A new ClusterShell release goes unnoticed until
  someone raises it, and raising it means recording the corpus again and
  listing or fixing whatever differs.
- A program that compares the package's folded output with ClusterShell's,
  as text, finds differences where names hold several numbers.
- The generator keeps out of the known differences, so a difference in a
  shape it does not generate, such as one host at two widths or whitespace,
  is caught only by the corpus.

## 7. The package prints what ClusterShell prints

Status: accepted, in part superseded by
[decision 8](#8-a-set-keeps-its-ranges-and-its-fold)

### Context

Decision 6 held the package to ClusterShell's hosts everywhere, and to its
printed output only where every host name has one number. It kept four
differences as the package's own: the folding of names with several numbers,
the order `Expand` lists them in, the source of a bare `@group` inside a group,
and an unknown group being an error. The maintainer asked for all four to go
before the first release, except that `exe[1-2-3]` stays an error rather
than naming hosts with negative numbers, as ClusterShell reads it.

### Decision

- A set whose names hold several numbers folds as ClusterShell's `RangeSetND`
  folds it: a set that is the product of its dimensions is one vector, and
  any other set is merged from its hosts in the passes ClusterShell makes,
  the vectors sorted before each pass as ClusterShell sorts them, first and
  last values compared as text. `String` writes the vectors in that order.
- `Expand`, and `Split` with it, list the hosts of such a pattern vector by
  vector, the last number varying fastest, as ClusterShell iterates a set.
- ClusterShell compares every pair of vectors in its last passes. The package
  finds the vectors that can merge through an index by all their dimensions
  but one, and takes them in ClusterShell's order, so the result is the same
  and a pass stays near linear. A test compares it with a plain transcription
  of ClusterShell's loop.
- A bare `@group` inside a group of a named source is resolved in that
  source.
- `MapResolver` answers a group it does not hold, and `@` alone, with no
  hosts, as a static source of ClusterShell does. The engine hands every
  reference to the resolver, an empty name included, and fails with any
  error a resolver returns, so a resolver for which an unknown group is an
  error keeps it one.
- The comparison with ClusterShell requires the same folded output and the
  same order of `Expand` for every generated expression, with groups that
  refer to groups of their own source bare and groups the sources do not
  hold.

### Costs

- The folded output, `Expand` and `Split` change for sets whose names hold
  several numbers. clusterctl v0.4.0, where the engine came from, prints the
  old form; a program that stored or compared it sees the new one. The module
  has no release yet, so no release of it changes.
- ClusterShell's folding can split a set into more vectors than the old one
  did, and it depends on details such as comparing numbers as text, which
  the package now reproduces rather than chooses.
- Folding a large set that is no product takes longer: up to twice the old
  fold's time on the sets measured, about two seconds for 950,000 hosts with
  holes in them, and `Expand` folds such a set before it lists it. The index
  and the sets a linear pass grows in are there to keep it near linear, and
  they are code of their own to keep right.
- A program that relied on `MapResolver` to refuse a mistyped group now
  selects nothing instead, unless it supplies a resolver of its own.

## 8. A set keeps its ranges and its fold

Status: accepted, extended by
[decision 9](#9-operators-combine-ranges-where-the-result-is-ranges)

### Context

A set held every host in one map, keyed by a string of its name, so
`r[1-1000]n[1-1000]` built a million entries before anything could be done
with it. ClusterShell keeps the ranges, and folded and listed that set in
milliseconds where the package took seconds. Every call of `String`,
`Hostlist` and `Expand` also grouped and sorted the hosts again, and
`Expand` folded a set with several numbers again to list it in
ClusterShell's order.

### Decision

- A set holds its hosts by pattern. The hosts of a pattern are either the
  product of the ranges a bracketed name gives, which is never listed host
  by host, or listed in a map keyed by their values. A product is listed
  only when an operator combines it with other hosts of its pattern, and
  two products are intersected range by range.
- The budget still counts the hosts a term names, whether or not they are
  listed.
- A set keeps the fold of each pattern until its hosts change. `Expand`
  and `Split` list the hosts from the fold, in ClusterShell's order, without
  sorting them again.
- A name without brackets is added to the set it is united with directly,
  without a set of its own.
- The fold of a set of one number sorts its values once, and the first
  pass of the fold of a set with several numbers sorts its hosts with a
  radix sort.
- A set may be read from several goroutines at once: the fold is stored
  atomically, and two readers that fold at the same time store the same
  fold.

### Costs

- The output is unchanged; the ClusterShell comparison, the corpus and the
  tests check it.
- A set that is read keeps its fold, which for a listed set of one number
  is a second copy of its values.
- A set has two forms of storage, and the operators handle both.

## 9. Operators combine ranges where the result is ranges

Status: accepted

### Context

Decision 8 kept a bracketed name as the product of its ranges, but any
operator other than an intersection listed the hosts of both operands one
by one. `exe[1-1000000]!exe5` took a second, and
`r[1-1000]n[1-1000]!r[1-10]n[1-1000]` too, where ClusterShell answered the
second in milliseconds. ClusterShell keeps a set of names with several
numbers as a list of vectors and combines them vector by vector, but folds
a result with holes host by host all the same.

### Decision

- Two products of one pattern combine range by range when the result is a
  product again: a union when one holds the other or the two differ in one
  dimension, spellings included; a difference when they share no host or
  the first holds values outside the second in one dimension only; an
  intersection always; a symmetric difference when they differ in one
  dimension at most. Every other combination lists the hosts, as before.
- A result is folded and listed from its hosts as before, so a product
  prints and lists as the same hosts listed one by one do.
- `Hostlist` writes a product from its ranges, without listing its hosts.
- A dimension stays one sorted slice of values. Keeping it as runs would
  make `exe[1-1000000]` cost less than its 16 MiB, but every fold, list
  and lookup would change with it, for no case the budget allows to cost
  more than a few tens of milliseconds.
- Operators on lists of vectors, as ClusterShell has them, are not taken
  up: a result with holes is folded from its hosts in ClusterShell as
  well.

### Costs

- Each operator has a second path, for two products, which the operators
  on listed hosts check against in the tests.
- A result with holes still lists the hosts of its pattern, and the fold
  of a million of them takes about two seconds.

## 10. The first release is v1.0.0

Status: accepted

### Context

Decision 4 kept the module at v0 until its API had held still through
releases of the programs that import it. The module has not been tagged
yet: the engine was written and released inside clusterctl, and its tags
stayed there. Its API is small, a set, its operators and a resolver, and
what it accepts and prints is held to ClusterShell's by the corpus and by
ClusterShell itself in CI (decisions 6 and 7), not left to settle. The
maintainer chose v1.0.0 for the first release.

### Decision

- The first release is v1.0.0, and the module follows semantic versioning
  from it. A fix is a patch release. New API, a new form of expression, a
  raised limit or a higher `go` line is a minor release.
- A minor or patch release does not break:
  - an exported identifier or its documented behaviour;
  - an expression it accepts, which keeps naming the same hosts;
  - the output of `String` and `Hostlist`, and the order of `Expand`.
- The text of an error, speed and memory are not covered. The expansion
  limits may be raised, never lowered.
- A breaking change is a new major version, with its own module path,
  `github.com/GSI-HPC/go-nodeset/v2`, and is a decision of its own.
- The rest of decision 4 stands: a release is a signed tag, nothing in the
  tree names a version, and a broken release is retracted.

### Costs

- An exported identifier stays until v2. `Resolver` cannot gain a method:
  a new capability comes as an optional interface, as `Lister` does.
- Recording a newer ClusterShell whose output differs changes the output
  of this package, so it waits for a major version, or is listed in
  `doc/language.md` as a divergence until then.
- A v2 changes the import path of every program that moves to it.

## 11. MapResolver evaluates every group of a source on its own

Status: accepted

### Context

`@source:*`, and `@*` for the default source, asks the resolver's `All` for
one expression, which the parser evaluates left to right like any other.
`MapResolver.All` joined the expressions of the source's groups with
commas, so an operator inside one group applied to every group joined
before it. With the groups `a: exe1` and `b: exe[2-4]!exe1`, `@*` named
`exe[2-4]` where `@a,@b` named `exe[1-4]`, although `Resolver` documents
`All` as naming every host the source knows.

ClusterShell 1.10.1 does the same for a source without an `all` group,
such as a YAML group file without one: it joins the groups' expressions
with commas and parses the result as one, so `nodeset -f '@*'` over that
file prints `exe[2-4]`. clusterctl met the bug in its copy of the engine
and fixed it there.

### Decision

- `MapResolver.All` writes a group into the union as its expression when
  that holds no operator but the union, and otherwise as the reference
  `@source:group`, which the parser evaluates on its own, at one more level
  of nesting. A source without such a group gives the same expression as
  before.
- A group that has to be referred to, but whose name the parser would not
  read back as that one reference, makes `All` an error rather than a
  guess: a name that is empty or `*`, or holds whitespace, a comma, `!`,
  `&`, `^` or a bracket, or a source name that holds one of those
  characters or `:`. An empty source name reads back as `@:group`.
- The doc of `Resolver.All` says that its expression is evaluated as one,
  so that another resolver that joins groups keeps each group's operators
  to that group.
- Here the package departs from ClusterShell: `@*` names what `Resolver`
  documents, the hosts of every group, rather than what ClusterShell's
  fallback for a source without `all` gives. `doc/language.md` says so
  under Groups; the corpus holds no groups, so its table of divergences
  cannot.

### Costs

- `@*` and `@source:*` over a `MapResolver` source with such a group name
  other hosts than they did in v1.0.0, and `All` returns another
  expression for that source, or an error.
- A program that reads one table of groups with both the package and
  ClusterShell gets different hosts for `@*` from the two, unless it gives
  ClusterShell what `All` returns as the source's `all` group, as
  `TestClusterShellOracle` does.
- No test compares the package with ClusterShell's fallback.
