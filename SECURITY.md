<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Security policy

## Supported versions

Fixes go into a new release of the newest minor version. While the module
is at v0, that is the only version supported.

## Reporting a vulnerability

Report it privately, through
[GitHub's private vulnerability reporting](https://github.com/GSI-HPC/go-nodeset/security/advisories/new),
not in a public issue. Say which version you used, what input or call shows
the problem, and what it lets an attacker do.

The maintainers answer in the advisory and work on the fix there. A fix is
released with a GitHub security advisory, which the Go vulnerability
database takes up, so that `govulncheck` reports the affected versions.
