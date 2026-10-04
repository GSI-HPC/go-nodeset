<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Node sets

A node set expression names a set of hosts. The syntax is ClusterShell's,
because that is what administrators of HPC clusters already type and what
their other tools accept. The semantics follow ClusterShell except in the
corners listed under
[Where this differs from ClusterShell](#where-this-differs-from-clustershell).

The Go package `nodeset` implements the language, and this document is its
reference: the syntax, the rules chosen where an implementation has to
choose, and the limits. The package documentation covers the API.

## Syntax

```
exe0001                   one host
exe[1-10]                 a range
exe[0001-0010]            a padded range
exe[1-10/2]               a range with a step
exe[1,5,9-12]             several ranges
rack[1-2]node[01-04]      two numeric dimensions
exe[1-4].hpc.example.org  a name with a domain
@compute                  a group
@slurm:main               a group from a named source
@*                        every host of the default source
```

Set operators combine expressions:

| Operator | Meaning |
| --- | --- |
| `,` or whitespace | union |
| `!` | difference |
| `&` | intersection |
| `^` | symmetric difference |

Operators have **no precedence**; an expression is evaluated strictly left to
right. `exe[1-10]!exe[1-5]&exe[1-7]` is `((exe[1-10] minus exe[1-5]) intersect
exe[1-7])`, which is `exe[6-7]`. Write the order you mean.

Whitespace unions, so the arguments of a command line can be joined with a
space and parsed in one call.

`!`, `&` and `^` need an operand on each side. `@rack:R02&`, `exe[1-10]!,exe5`,
`exe[1-10]&&exe5` and `!exe5` are errors. The first is what
`"@rack:R02&$(idle-nodes)"` leaves behind when a command that lists the idle
nodes prints none, and reading it as `@rack:R02` would select the whole rack
instead of nothing. A union tolerates an empty operand, because a union with nothing is
what was meant: `exe1,` is `exe1`.

A range needs both its bounds. `exe[1-]` and `exe[1-,5]` are errors: they are
what `exe[1-$N]` leaves behind when `N` is empty, and reading the first as
`exe1` would select one host where several were meant.

## Semantics chosen here

These are the corners where an implementation has to decide something. They are
written down because the behaviour is observable.

### Every run of digits is a dimension

`x1y1` has two numeric dimensions, `10.0.1.7` has four, and `exe0001` has one,
whether or not brackets were written. A dimension holding a single value is
rendered without brackets, so folding is idempotent: the printed form of a set
parses back into the same set, every host spelled as before, and prints the
same way again. This property is checked by a fuzz test.

### Padding is not part of a host's identity

**Decision:** names that differ only in zero padding, such as `exe1`, `exe01`
and `exe0001`, are **one host**. `exe1,exe01` names one host, `exe[1-3]!exe02`
is `exe[1,3]`, and a set holding `exe0001` contains `exe1`.

This is what lets someone type `exe1` and reach the machine a list of hosts
wrote as `exe0001`. The price is that one set cannot hold two hosts whose names
differ only in padding: they are one member. A set does not report that it was
given one host under two spellings, so a program for which that is an error,
such as two machines in an inventory, checks its names itself before it builds
a set.

Padding is still never thrown away. Each host keeps the spelling it was first
given, and a set never shows a host under a name it was not given:

- `exe1,exe01` prints `exe1`, and `exe01,exe1` prints `exe01`.
- `exe[01-02]` plus `exe3` prints `exe[01-02,3]`, not `exe[01-03]`.
- A set holding `exe[0001-0010]` and `exe11` prints `exe[0001-0010,11]`, and
  `Canonical("exe11")` answers `exe11`.
- A value only joins a range when it reads the same at the range's width, so
  `exe08,exe09,exe10` prints `exe[08-10]` but `exe7,exe08,exe9` prints
  `exe[7,08,9]`.

In a range the width of the first bound applies to the whole range:
`exe[01-100]` is `exe01` to `exe99` and `exe100`. A last bound padded to
another width, as in `exe[1-010]` or `exe[001-10]`, is an error, because one of
the two bounds would be shown under a name it was not written as.

### Adjacent numeric parts are rejected

`exe0[0,10]` is an error. It expands to `exe00` and `exe010`, and neither name
can be split back into the same two dimensions, so folding it would silently
lose a host. Separate numeric parts with a literal character.

### A name cannot begin with a dash

`-oProxyCommand=x` is an error. No host name begins with `-`, and ssh and most
other tools a name is handed to would read one as an option.

### Otherwise a name is not checked

The language accepts more than a host name may contain, because a set is also
used for things that are not hosts: of the host name rules, the parser
enforces only that a name does not begin with `-`. A program that hands a name
to ssh or puts it in a URL, where `:`, `@`, `/`, `?` and `#` mean something,
checks it against the rules for host names first, the names a group resolves
to included.

### Steps are read but not written

`exe[1-10/2]` parses. Folding does not produce a step unless it is asked for,
which is what ClusterShell's `--autostep` does; without it, `exe[1,3,5,7]`
prints as it is. With `WithAutostep(n)` a run of at least n values with one
step is written as a range with that step, and the values are taken from left
to right as ClusterShell takes them, so that both write a set of one
dimension alike: `node[154,176]` with a threshold of two is
`node[154-176/22]`, and `exe[1,3,5,6]` with three is `exe[1-5/2,6]`.

## Where this differs from ClusterShell

ClusterShell 1.10.1 was run over the corpus in
`testdata/clustershell.txt` next to the package, and a test, which
[testing.md](testing.md#what-is-tested-where) describes, checks that the
package agrees with it on every other line of that corpus and differs on
these. Each row ends with the name, in brackets, that marks its lines in the
corpus, and the test checks that this table names every divergence it knows
and no other:

| Expression | ClusterShell | `nodeset` |
| --- | --- | --- |
| `exe1,exe01` | two hosts, `exe[1,01]` | one host, `exe1` (padding identity) |
| `exe[1-3]!exe02` | `exe[1-3]` | `exe[1,3]` (padding identity) |
| `exe[01-100]` | error: padding length mismatch | `exe01` to `exe99` and `exe100` (padding mismatch) |
| `exe0[0,10]` | `exe00`, `exe010` | an error (adjacent numeric parts) |
| `exe[1-3] sub1` | whitespace is part of the name | union, `exe[1-3],sub1` (whitespace) |
| `exe[1-3],` and `exe1,,exe2` | error | the empty operand is nothing (empty operand) |
| `-oProxyCommand=x` | accepted as a name | an error (leading dash) |
| `exe[1-2-3]` | the hosts `exe-2` to `exe3` | an error (malformed range) |

Both reject `exe[1-010]`, `exe[001-10]`, a range without its last bound such
as `exe[1-]` or `exe[1-,5]`, a dangling `!`, `&` or `^`, and a set operator
with no left operand. On the other lines of the corpus both name the same
hosts. Folded output may still be ordered differently: ClusterShell prints
`exe[3,01-02]` where the package prints `exe[01-02,3]`.

Beyond the corpus, a test runs ClusterShell itself over thousands of
generated expressions ([testing.md](testing.md#what-is-tested-where)). Apart
from the rows above, both name the same hosts for every one of them, print
them folded character for character alike, and list them in the same order.

The corpus holds no groups. `@*` over a source of `MapResolver` differs from
what ClusterShell gives for a source without an `all` group when a group
holds `!`, `&` or `^`, or its brackets do not balance, as [Groups](#groups)
says.

With several numbers in a name there is more than one way to fold a set. The
package folds it the way ClusterShell does: a set that is the product of its
dimensions is one vector, and any other set is merged from its hosts in
passes, the vectors ordered as ClusterShell orders them. `Expand` lists the
hosts vector by vector, as ClusterShell does; within a pattern with one
number it lists them in numeric order.

## Groups

`@group` and `@source:group` are resolved by a `Resolver` the program
supplies; parsing without one rejects every group reference. The source is
handed over as it was written, empty for a bare `@group`, so the resolver
decides what that means: its default source, or a search of several. `@*` and
`@source:*` ask it for every host of a source.

The resolver answers `@*` with one expression, which is evaluated left to
right like any other, so a resolver that joins the expressions of several
groups keeps each group's operators to that group. `MapResolver` gives the
union of the source's groups, each evaluated on its own, as `@a,@b` evaluates
them: a group whose expression holds `!`, `&` or `^`, or whose brackets do
not balance, goes in as the reference `@source:group`, one level of nesting
deeper. Such a group is an error when its name is empty or `*`, or holds
whitespace, a comma, one of those operators or a bracket, or when its
source's name holds one of those or `:`, since that reference might not read
back as the group. Here ClusterShell differs: for a source without an `all`
group it joins the groups' expressions into one, so with `a: exe1` and
`b: exe[2-4]!exe1` its `@*` is `exe[2-4]`, where the package's is `exe[1-4]`,
and with `a: exe[1` and `b: 3]` it is `exe[1,3]`, where the package's is an
error ([decision 11](decisions.md#11-mapresolver-evaluates-every-group-of-a-source-on-its-own),
[decision 12](decisions.md#12-a-group-with-unbalanced-brackets-is-evaluated-on-its-own-too)).

The expression a resolver returns is parsed in turn, with the same resolver,
so a group may refer to other groups. Nesting is cut off after sixteen levels,
which also reports a cycle rather than looping. The hosts of every group an
expression refers to count towards its [limits](#limits).

A bare `@group` inside a group of a named source is looked up in that
source, as ClusterShell does: a group of source `ib` that refers to `@fabric`
means `@ib:fabric`. A bare `@group` anywhere else is handed to the resolver
with an empty source.

What a group the resolver does not know means is the resolver's to decide.
`MapResolver` answers it with no hosts, as ClusterShell's static sources do,
and answers `@` alone the same way. A program for which a mistyped group
must not select nothing supplies a resolver that returns an error, and
parsing fails with it.

## Rendering for other tools

`NodeSet.String()` gives the folded form. For Slurm or FreeIPMI,
`NodeSet.Hostlist()` gives a host list instead, which folds each pattern
along the one dimension that gives the fewest names, the last one on a tie, and
never writes a step. `rack[1-2]node[001-100]` becomes
`rack1node[001-100],rack2node[001-100]`, and a BMC name such as
`exe[0001-4600].mgmt.dc2.example.org` stays one name rather than 4,600.

Every name then carries at most one bracketed range. That is the form every
version of the two host list parsers reads. `scontrol` 23.11 and `ipmipower`
1.6.13 also read several ranges in one name, but older ones are not known to,
and the one-range form is short enough: it grows with the number of values of
the other dimensions, not with the number of hosts. Neither reads `@groups`,
which are resolved before a set is rendered.

## Limits

A bracket is capped at 2²⁰ elements, counting all of its parts together, and
an expression at 2²⁰ hosts. The expression cap holds at every step of the
evaluation, so `a[1-600000],b[1-600000]!b[1-600000]` is refused even though it
ends up smaller. It also holds for all the terms of an expression together,
groups and the groups they refer to included, so the work an expression costs
is bounded as well as its result: `a[1-600000]!a[1-600000],b1` is refused too.

Each dimension of a name is weighed before it is expanded, against what the
dimensions before it and the terms before it have left, so an oversized
expression is refused before its memory is spent. A typo such as
`exe[1-100000000]` is reported rather than exhausting memory. A bracketed
name is kept as its ranges, so the caps count hosts named, not memory spent.
Two such names of one pattern combine range by range whenever the result is
the product of ranges again: in a union when one holds the other or the two
differ in one dimension, in a difference when they share no host or the
first has hosts outside the second in one dimension only, in an intersection
always, and in a symmetric difference when they differ in one dimension at
most. `exe[1-1000000]!exe5` and `r[1-1000]n[1-1000]!r[1-10]n[1-1000]` never
list their hosts. Any other combination lists the hosts of the pattern one
by one. The most an expression can cost is 2²⁰ names written one by one:
parsing them allocates about 215 MiB and takes one to one and a half seconds
on a four-core machine.

Bounds and steps are plain decimal numbers of at most eighteen digits, so no
arithmetic on them can overflow.

A set keeps its fold until it changes, and `Expand` lists the hosts from
it. A bracketed name folds and lists in time proportional to its ranges and
its hosts, and never sorts the hosts. Folding a listed set of one number costs
O(n log n) in the number of hosts. A listed set with several numbers in its
names is folded in passes, each of which costs about O(n log n); ClusterShell
compares every pair of vectors in some of them, and the package finds the
same pairs through an index instead. Parsing and printing a million hosts
written one by one takes about two seconds when their names have one number,
and about four and a half when they have two.
