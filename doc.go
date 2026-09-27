// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

// Package nodeset parses, folds and expands [ClusterShell] style node sets,
// such as exe[0001-0010/2] or rack[1-2]node[01-04]!@drained.
//
// The engine is still being moved here; until then this package holds no
// API.
//
// [ClusterShell]: https://clustershell.readthedocs.io/en/latest/tools/nodeset.html
package nodeset
