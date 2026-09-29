#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
# SPDX-License-Identifier: Apache-2.0
"""Answer node set expressions with ClusterShell, for TestClusterShellOracle.

The first line of standard input names the group sources, as a JSON object:

    {"default": "site",
     "sources": {"site": {"compute": "exe[1-4]"}},
     "all": {"site": "exe[1-4]"}}

Every further line is a request, {"expr": "exe[1-4]", "autostep": 0}. For
each the script writes one line of JSON: the version of ClusterShell first,
then {"hosts": [...], "folded": "..."} with the hosts in ClusterShell's order
and its folded form, or {"error": "..."} when ClusterShell rejects the
expression. An autostep of 0 means none.

Install the version in requirements.txt next to this script to run it.
"""

import json
import sys

import ClusterShell
from ClusterShell.NodeSet import NodeSet
from ClusterShell.NodeUtils import GroupResolver, GroupSource


def resolver(config):
    sources = {
        name: GroupSource(name, groups=groups, allgroups=config["all"][name])
        for name, groups in config["sources"].items()
    }
    res = GroupResolver(sources[config["default"]])
    for name, source in sources.items():
        if name != config["default"]:
            res.add_source(source)
    return res


def answer(res, request):
    try:
        ns = NodeSet(request["expr"], resolver=res,
                     autostep=request["autostep"] or None)
        return {"hosts": list(ns), "folded": str(ns)}
    except Exception as err:  # every rejection is an answer
        return {"error": "%s: %s" % (type(err).__name__, err)}


def main():
    res = resolver(json.loads(sys.stdin.readline()))
    out = sys.stdout
    out.write(json.dumps({"version": ClusterShell.__version__}) + "\n")
    for line in sys.stdin:
        out.write(json.dumps(answer(res, json.loads(line))) + "\n")


if __name__ == "__main__":
    main()
