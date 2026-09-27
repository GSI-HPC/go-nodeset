<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# go-nodeset documentation

This directory records how the module is built and why, for someone who
changes it. How to use the module is in its doc comments and examples,
which pkg.go.dev publishes:
<https://pkg.go.dev/github.com/GSI-HPC/go-nodeset>.

| Document | What it covers |
| --- | --- |
| [decisions.md](decisions.md) | What was decided, why, and what it costs |
| [release.md](release.md) | Cutting, withdrawing and verifying a release |

Two more arrive with the engine:

- `language.md`: the node set language, the rules chosen and why, where
  they differ from ClusterShell, host lists for Slurm, and the limits;
- `testing.md`: what is tested and how, from fuzzing to the ClusterShell
  corpus and the size tests.

The measured comparison with other Go node set libraries arrives with them,
as a decision in `decisions.md`.

## Everything else

| For | Where |
| --- | --- |
| A program that uses the module | The doc comments and examples, on pkg.go.dev, and the [README](../README.md) for installing and versions |
| A contributor, person or agent | [AGENTS.md](../AGENTS.md): commands, conventions and rules |
| Someone reporting a vulnerability | [SECURITY.md](../SECURITY.md) |
| Someone citing the module | [CITATION.cff](../CITATION.cff) |
| Someone reading release notes | The signed tags, published as [GitHub releases](https://github.com/GSI-HPC/go-nodeset/releases) |

## Keeping it true

- The examples compile and run with the tests, and golangci-lint refuses an
  exported name without a doc comment.
- A change of behaviour updates the doc comments, and the document here that
  describes it, in the same pull request.
- A decision is never edited; a later one supersedes it.
- Once `language.md` is here, the ClusterShell corpus test checks its list of
  differences: a difference that goes away fails the test.

Deliberately absent: a documentation site, a changelog, a `CONTRIBUTING.md`
and a README per package. pkg.go.dev, the signed tags and `AGENTS.md` say
what they would.
