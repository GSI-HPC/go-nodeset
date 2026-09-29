<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Testing the node set engine

The `nodeset` package is tested on its own, with the standard library alone:
table-driven unit tests, a fuzz target, a differential corpus recorded with
ClusterShell, and tests of size and cost. [language.md](language.md) describes
the language they hold the package to.

## What is tested where

**Unit tests** cover parsing, folding and expansion, the set operators and
their order, groups through a table resolver, host lists, splitting, the
limits, and the spelling a set keeps for a host written with other padding.
The case name says which property is checked.

**A fuzz target**, `FuzzParseFold`, checks the two properties a node set
expression must satisfy: parsing never panics, and folding is idempotent. It
compares the hosts before and after folding name by name, so a fold that
renamed `exe3` to `exe03` would be caught, which a padding-blind membership
check would not. It also checks that `Hostlist` names the same hosts. It is
how the adjacent-numeric-parts ambiguity was found. A fuzzing worker gives up
on any input that runs for ten seconds, so the target lowers the expansion
limits to 2¹² and skips inputs longer than a kilobyte: every input stays
cheap, and the time goes into variety. CI runs it for a bounded time on every
change; locally, run it from the package's directory:

```console
$ go test -run '^$' -fuzz FuzzParseFold -fuzztime 60s .
```

**A differential corpus** holds node set expressions with the answer
ClusterShell gave for each, in `testdata/clustershell.txt`. A test checks that
the package names the same hosts, except on the lines marked as one of the
divergences [language.md](language.md#where-this-differs-from-clustershell)
lists, where it checks that the answers still differ, so that a divergence
which goes away is noticed and taken out of the documentation too. A second
test reads the table of divergences in `doc/language.md` and checks that it
names each divergence the corpus test knows, and no other.
`clustershell.py` next to it records the answers again, with ClusterShell
installed; the version it used is in the corpus header.

**ClusterShell itself** is the reference in a test that CI runs with the
ClusterShell `testdata/requirements.txt` pins. `TestClusterShellOracle` hands
expressions to `testdata/clustershell_oracle.py`, which answers them with
ClusterShell's `NodeSet`, and compares the answers with the package's:

- the lines of the corpus that name no divergence;
- expressions both must reject;
- 20,000 expressions generated from a fixed seed, of one to four terms joined
  by the four operators: ranges, steps, lists, padding, one to three numbers
  in a name, domains, nested groups from two sources, and autostep from 2
  to 4.

Both must reject the same expressions and name the same hosts, and each must
read what the other prints, the host list included, as the same hosts;
`String` must be ClusterShell's folded form and `Expand` must list the hosts
in ClusterShell's order. The groups refer to groups of their own source bare,
and the generated expressions include groups the sources do not hold. They
stay out of the differences
[language.md](language.md#where-this-differs-from-clustershell) lists: each
number of a name has one width in every expression.

**The folding of names with several numbers** is checked against a
reference as well. `TestFoldNDMatchesReference` folds random sets of two to
four dimensions, some of them with values at two widths, both with the
package and with a plain transcription of ClusterShell's `RangeSetND`
folding, which compares every pair of vectors, and requires the same vectors
in the same order. Locally, `make clustershell` sets up ClusterShell in
`.clustershell/`, runs the test, and records the corpus again, failing if
ClusterShell's answers have changed.
`NODESET_CLUSTERSHELL_CASES` and `NODESET_CLUSTERSHELL_SEED` run more
expressions, or others. The test is skipped where `NODESET_CLUSTERSHELL_PYTHON`
is not set, so that `go test` needs nothing but Go.

**Size and cost** are tests of their own. One folds sets of a quarter of a
million hosts, which would take more than a minute if folding were quadratic.
One lowers the limits to a hundred and checks that every way of exceeding them
is refused, a range, a pattern, several terms together and groups included.
One measures what two oversized expressions allocate before they are refused,
and holds it to what the largest permitted set costs. Tests that lower the
limits or measure allocation change or read state of the whole package, so
they do not run in parallel.

## Coverage

Every statement of the package is covered. A branch no test can reach is
deleted rather than excluded, and a test asserts on behaviour that would be
wrong if it changed, not on the shape of the implementation.
