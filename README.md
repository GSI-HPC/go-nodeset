<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# go-nodeset

[![Go Reference](https://pkg.go.dev/badge/github.com/GSI-HPC/go-nodeset.svg)](https://pkg.go.dev/github.com/GSI-HPC/go-nodeset)
[![CI](https://github.com/GSI-HPC/go-nodeset/actions/workflows/ci.yml/badge.svg)](https://github.com/GSI-HPC/go-nodeset/actions/workflows/ci.yml)

**[ClusterShell](https://clustershell.readthedocs.io/en/latest/tools/nodeset.html)
node sets for Go.** Parse, fold and expand ranges and steps
(`exe[0001-0010/2]`), several numeric dimensions (`rack[1-2]node[01-04]`),
the set operators `,` `!` `&` `^`, and `@group` references through a
resolver of your own. Print sets folded, or as host lists that
[Slurm](https://slurm.schedmd.com/) accepts. The standard library is its
only dependency.

**[API reference on pkg.go.dev →](https://pkg.go.dev/github.com/GSI-HPC/go-nodeset)**

## Status

The engine was written as the `nodeset` package of
[clusterctl](https://github.com/GSI-HPC/clusterctl), a command-line tool for
administering HPC clusters, and moved here with its history once it had
shipped in a clusterctl release. Why it is an engine of its own rather than
an existing Go library is
[decision 5](doc/decisions.md#5-an-engine-of-its-own-rather-than-an-existing-go-library).

## Usage

```go
import "github.com/GSI-HPC/go-nodeset"

ns, err := nodeset.Parse("rack[1-2]node[01-04]!rack1node02")
if err != nil {
	return err
}
fmt.Println(ns)            // rack[1-2]node[01,03-04],rack2node02
fmt.Println(ns.Len())      // 7
fmt.Println(ns.Hostlist()) // rack1node[01,03-04],rack2node[01-04], for Slurm

// @group references go through a resolver of your own.
groups := nodeset.NewMapResolver("site", map[string]string{"gpu": "rack2node[03-04]"})
cpu, err := nodeset.ParseWith("rack[1-2]node[01-04]!@gpu", groups)
if err != nil {
	return err
}
fmt.Println(cpu) // rack[1-2]node[01-02],rack1node[03-04]
```

The language, and where it differs from ClusterShell, is described in
[`doc/language.md`](doc/language.md); the API, with more examples, on
[pkg.go.dev](https://pkg.go.dev/github.com/GSI-HPC/go-nodeset).

## Install

```console
$ go get github.com/GSI-HPC/go-nodeset
```

It needs Go 1.26 or newer: the module requires the oldest Go release the Go
project still supports, and follows it up after each Go release
([decision 2](doc/decisions.md#2-the-go-line-is-the-oldest-go-release-still-supported)).

## Versions

Releases are signed tags `vX.Y.Z`, and each has
[release notes](https://github.com/GSI-HPC/go-nodeset/releases). The module
is at v0 while its API settles, so a minor release may change it; its notes
say how ([decision 4](doc/decisions.md#4-a-release-is-a-signed-tag)).

## Contributing

The repository carries a `mise.toml`, so the toolchain comes from
[mise](https://mise.jdx.dev) if you use it:

```console
$ mise install    # Go and golangci-lint, at the versions CI uses
$ make lint       # golangci-lint
$ make test       # tests under the race detector
$ make cover      # every file at 100% coverage
$ make help       # every other check CI runs
```

Commits are [Conventional Commits](https://www.conventionalcommits.org/).
How the module is built and why is in [`doc/`](doc/), and a change that
takes a decision adds it to [`doc/decisions.md`](doc/decisions.md).
Instructions for coding agents are in
[`AGENTS.md`](AGENTS.md). Report vulnerabilities as
[`SECURITY.md`](SECURITY.md) says.

## AI disclosure

This project is developed with the help of AI coding tools. Changes written
by Anthropic's Claude Code agent are committed as
`Claude <noreply@anthropic.com>` and/or carry a `Co-Authored-By: Claude …`
trailer.

## Licence

Copyright 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH
<http://www.gsi.de>

Apache-2.0. See [`LICENSE`](LICENSE).
