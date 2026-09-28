// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset_test

import (
	"fmt"
	"strings"

	"github.com/GSI-HPC/go-nodeset"
)

func ExampleParse() {
	ns, err := nodeset.Parse("exe[0001-0010]!exe0003")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ns, ns.Len())

	// Operators have no precedence: the expression is read from left to
	// right.
	ns, err = nodeset.Parse("exe[1-10]!exe[1-5]&exe[1-7]")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ns)

	_, err = nodeset.Parse("exe[1-10]&")
	fmt.Println(err)
	// Output:
	// exe[0001-0002,0004-0010] 9
	// exe[6-7]
	// in "exe[1-10]&": the & operator has no right operand
}

func ExampleNodeSet_String() {
	// Hosts written one by one fold into ranges. exe3 is exe03 again, since
	// padding is not part of a host's identity, and a host keeps the spelling
	// it was first given.
	ns := nodeset.MustParse("exe01,exe02,exe03,exe3 exe05 login")
	fmt.Println(ns)

	// Several numeric dimensions fold independently.
	fmt.Println(nodeset.MustParse("rack1node01,rack1node02,rack2node01,rack2node02"))

	// Steps are written only when asked for.
	fmt.Println(nodeset.MustParse("exe[1-9/2]"))
	fmt.Println(nodeset.MustParse("exe[1-9/2]", nodeset.WithAutostep(3)))
	// Output:
	// exe[01-03,05],login
	// rack[1-2]node[01-02]
	// exe[1,3,5,7,9]
	// exe[1-9/2]
}

func ExampleNodeSet_Expand() {
	// Hosts come in order of their numbers, not of their names as strings.
	ns := nodeset.MustParse("exe[9-11],rack[1-2]node[1-2]")
	fmt.Println(strings.Join(ns.Expand(), " "))
	// Output:
	// exe9 exe10 exe11 rack1node1 rack1node2 rack2node1 rack2node2
}

func ExampleNodeSet_Hostlist() {
	// For Slurm and FreeIPMI: one bracketed range per name at most, and no
	// steps.
	ns := nodeset.MustParse("rack[1-2]node[001-100],exe[1-9/2]")
	fmt.Println(ns)
	fmt.Println(ns.Hostlist())
	// Output:
	// exe[1,3,5,7,9],rack[1-2]node[001-100]
	// exe[1,3,5,7,9],rack1node[001-100],rack2node[001-100]
}

func ExampleParseWith() {
	// A group may refer to other groups; the resolver's answer is parsed in
	// turn.
	res := nodeset.NewMapResolver("site", map[string]string{
		"compute": "exe[0001-0100]",
		"gpu":     "exe[0097-0100]",
		"cpu":     "@compute!@gpu",
	})

	ns, err := nodeset.ParseWith("@cpu&exe[0090-0200]", res)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ns)

	// @* asks the resolver for every host of its default source.
	ns, err = nodeset.ParseWith("@*", res)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ns.Len())

	_, err = nodeset.ParseWith("@login", res)
	fmt.Println(err)
	// Output:
	// exe[0090-0096]
	// 100
	// group @login: unknown group "login" in source "site"
}
