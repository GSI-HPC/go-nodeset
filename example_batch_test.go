// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset_test

import (
	"fmt"
	"strings"

	"github.com/GSI-HPC/go-nodeset"
)

// groupSource is a group source whose every lookup is a round trip, such as
// one that runs a command or asks a directory, so it would rather look up
// several groups at once. A table stands in for the source here, and each
// lookup is printed.
type groupSource struct {
	*nodeset.MapResolver
}

// Resolve looks up one group.
func (s groupSource) Resolve(source, group string) (string, error) {
	fmt.Println("Resolve", ref(source, group))
	return s.MapResolver.Resolve(source, group)
}

// ResolveBatch looks up several groups at once: in one request, such as an
// LDAP query with an OR filter, or in parallel.
func (s groupSource) ResolveBatch(refs []nodeset.GroupRef) []nodeset.GroupAnswer {
	names := make([]string, len(refs))
	answers := make([]nodeset.GroupAnswer, len(refs))
	for i, r := range refs {
		names[i] = ref(r.Source, r.Group)
		answers[i].Expr, answers[i].Err = s.MapResolver.Resolve(r.Source, r.Group)
	}
	fmt.Println("ResolveBatch", strings.Join(names, " "))
	return answers
}

// ref writes a group reference as an expression does.
func ref(source, group string) string {
	if source == "" {
		return "@" + group
	}
	return "@" + source + ":" + group
}

func ExampleBatchResolver() {
	res := groupSource{&nodeset.MapResolver{
		Default: "site",
		Groups: map[string]map[string]string{
			"site": {"compute": "@rack:r1,@rack:r2", "gpu": "@rack:r3"},
			"rack": {"r1": "exe[01-04]", "r2": "exe[05-08]", "r3": "gpu[1-2]"},
		},
	}}

	// The groups an expression refers to are looked up together before
	// it is evaluated: those of the expression given, then those of each
	// group's answer. gpu's answer refers to one group, which Resolve
	// looks up.
	ns, err := nodeset.ParseWith("@compute,@gpu", res)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ns)
	// Output:
	// ResolveBatch @compute @gpu
	// ResolveBatch @rack:r1 @rack:r2
	// Resolve @rack:r3
	// exe[01-08],gpu[1-2]
}
