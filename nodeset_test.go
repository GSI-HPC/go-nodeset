// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset_test

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/GSI-HPC/go-nodeset"
)

func TestParseExpand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expr string
		want []string
	}{
		{"exe0001", []string{"exe0001"}},
		{"login", []string{"login"}},
		{"exe[1-3]", []string{"exe1", "exe2", "exe3"}},
		{"exe[0001-0003]", []string{"exe0001", "exe0002", "exe0003"}},
		{"exe[1-10/3]", []string{"exe1", "exe4", "exe7", "exe10"}},
		{"exe[1,5,9]", []string{"exe1", "exe5", "exe9"}},
		// Names with several numbers are listed vector by vector, as the set
		// folds and as ClusterShell lists them, not in numeric order.
		{"rack[1-2]node[01-04]!rack1node02", []string{"rack2node01", "rack2node02", "rack2node03", "rack2node04", "rack1node01", "rack1node03", "rack1node04"}},
		{"exe[1-3],rack[1-2]node[1-2]!rack1node1,sw1p[1-2],sw2p1", []string{"exe1", "exe2", "exe3", "rack2node1", "rack2node2", "rack1node2", "sw1p1", "sw1p2", "sw2p1"}},
		{"exe[09-11]", []string{"exe09", "exe10", "exe11"}},
		{"exe[1-2]-ib[0-1]", []string{"exe1-ib0", "exe1-ib1", "exe2-ib0", "exe2-ib1"}},
		{"rack[1-2]node[01-02]", []string{"rack1node01", "rack1node02", "rack2node01", "rack2node02"}},
		{"exe1.hpc.example.org", []string{"exe1.hpc.example.org"}},
		{"exe[1-2].hpc.example.org", []string{"exe1.hpc.example.org", "exe2.hpc.example.org"}},
		{"exe[1-3],sub[1-2]", []string{"exe1", "exe2", "exe3", "sub1", "sub2"}},
		{"exe[1-3] sub1", []string{"exe1", "exe2", "exe3", "sub1"}},
		{"exe[1-5]!exe[2-3]", []string{"exe1", "exe4", "exe5"}},
		{"exe[1-5]&exe[4-8]", []string{"exe4", "exe5"}},
		{"exe[1-3]^exe[2-4]", []string{"exe1", "exe4"}},
		{"exe[1-3]!exe2,sub1", []string{"exe1", "exe3", "sub1"}},
		{"", nil},
		// Padding is not part of a host's identity, so these name one host,
		// shown the way it was first written.
		{"exe1,exe01", []string{"exe1"}},
		{"exe01,exe1", []string{"exe01"}},
		{"exe[1,01]", []string{"exe1"}},
		// Every host keeps its own width, so none is shown under a name it
		// was not given.
		{"exe[01-02],exe3", []string{"exe01", "exe02", "exe3"}},
		{"exe[0001-0002,11]", []string{"exe0001", "exe0002", "exe11"}},
		{"exe[01-100/99]", []string{"exe01", "exe100"}},
		{"exe[001-010]", []string{"exe001", "exe002", "exe003", "exe004", "exe005", "exe006", "exe007", "exe008", "exe009", "exe010"}},
		{"exe[08-10]!exe09", []string{"exe08", "exe10"}},
		{"exe1 & exe[1-2]", []string{"exe1"}},
		{"exe1^sub1", []string{"exe1", "sub1"}},
		{"exe[1-3],sub1&exe2", []string{"exe2"}},
		{"exe[1-3]!sub1", []string{"exe1", "exe2", "exe3"}},
		{"exe[1-3]!exe[0-10]", nil},
		{"r[1-2]n[1-2]&r[2-3]n[3-4]", nil},
		// Two products of one pattern combine range by range when the
		// result is a product again, and are listed host by host when it
		// is not.
		{"exe[1-3],exe[01-02]", []string{"exe1", "exe2", "exe3"}},
		{"exe[1-3]^exe[01-03]", nil},
		{"exe[1-2],exe[04-05]", []string{"exe1", "exe2", "exe04", "exe05"}},
		{"r[1-2]n[1-2],r[3]n[1-2]", []string{"r1n1", "r1n2", "r2n1", "r2n2", "r3n1", "r3n2"}},
		{"r[1-2]n[1-2],r3n3", []string{"r1n1", "r1n2", "r2n1", "r2n2", "r3n3"}},
		{"r[1-2]n[1-2]!r[3-4]n[1-2]", []string{"r1n1", "r1n2", "r2n1", "r2n2"}},
		{"r[1-2]n[1-2]!r[1-2]n[1-5]", nil},
		{"r[1-3]n[1-2]!r2n[1-2]", []string{"r1n1", "r1n2", "r3n1", "r3n2"}},
		{"r[1-2]n[1-2]!r1n1", []string{"r2n1", "r2n2", "r1n2"}},
		{"r[1-2]n[1-2]&r[2-3]n[2-3]", []string{"r2n2"}},
		{"r[1-2]n[1-2]^r[1-2]n[1-2]", nil},
		{"r[1-2]n[1-2]^r[1-2]n[2-3]", []string{"r1n1", "r1n3", "r2n1", "r2n3"}},
		{"r[1-2]n[1-2]^r[2-3]n[2-3]", []string{"r1n1", "r1n2", "r2n1", "r2n3", "r3n2", "r3n3"}},
		{"login^login", nil},
		// Listed hosts combine with a product host by host.
		{"exe1,exe5!exe[1-3]", []string{"exe5"}},
		{"exe1,exe2,exe3!exe2", []string{"exe1", "exe3"}},
		{"exe1,exe2&exe2", []string{"exe2"}},
		{"exe1,exe2^exe[2-3]", []string{"exe1", "exe3"}},
		// A name without numbers is one host however often it is named.
		{"login,login", []string{"login"}},
		{"login login&login", []string{"login"}},
		{"login,login^login", nil},
		{"exe[1-3],,exe5,", []string{"exe1", "exe2", "exe3", "exe5"}},
		// A percent sign is an ordinary character of a name.
		{"a%b", []string{"a%b"}},
		{"50%[1-2]", []string{"50%1", "50%2"}},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			ns, err := nodeset.Parse(tc.expr)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.expr, err)
			}
			got := ns.Expand()
			if !equal(got, tc.want) {
				t.Errorf("Parse(%q).Expand() = %v, want %v", tc.expr, got, tc.want)
			}
			if ns.Len() != len(tc.want) {
				t.Errorf("Len() = %d, want %d", ns.Len(), len(tc.want))
			}
		})
	}
}

func TestFold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		want string
	}{
		{"single host keeps its name", "exe1", "exe1"},
		{"non numeric host is untouched", "login", "login"},
		{"contiguous run folds", "exe1 exe2 exe3", "exe[1-3]"},
		{"padding is preserved", "exe0001 exe0002", "exe[0001-0002]"},
		{"gaps are listed", "exe1 exe2 exe5", "exe[1-2,5]"},
		{"pads do not merge across widths", "exe09 exe10", "exe[09-10]"},
		{"the first spelling of a host wins", "exe1 exe01", "exe1"},
		{"every host keeps its width", "exe[01-02] exe3", "exe[01-02,3]"},
		{"a stray unpadded host is not renamed", "exe[0001-0010] exe11", "exe[0001-0010,11]"},
		{"a wider value joins a padded run", "exe08 exe09 exe10 exe11", "exe[08-11]"},
		{"widths split a run", "exe7 exe08 exe9", "exe[7,08,9]"},
		{"a wider value ends a stepped run", "exe1 exe3 exe05", "exe[1,3,05]"},
		{"patterns are listed alphabetically", "sub1 exe1", "exe1,sub1"},
		{"a single valued dimension loses its brackets", "exe[1-1]", "exe1"},
		{"two dimensions fold into one vector", "exe1-ib0 exe1-ib1 exe2-ib0 exe2-ib1", "exe[1-2]-ib[0-1]"},
		// A set that is no product folds as ClusterShell folds it: the hosts
		// in numeric order, merged with their neighbours first.
		{"a ragged set folds as ClusterShell folds it", "exe1-ib0 exe1-ib1 exe2-ib0", "exe1-ib[0-1],exe2-ib0"},
		{"the larger vector comes first", "cn[001-032]-ib[0-1]!cn[001-004]-ib0", "cn[005-032]-ib[0-1],cn[001-004]-ib1"},
		{"domains fold on the host part", "exe1.hpc.example.org exe2.hpc.example.org", "exe[1-2].hpc.example.org"},
		{"an empty set renders empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ns, err := nodeset.Parse(tc.expr)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.expr, err)
			}
			if got := ns.String(); got != tc.want {
				t.Errorf("Parse(%q).String() = %q, want %q", tc.expr, got, tc.want)
			}
		})
	}
}

func TestFoldRoundTrip(t *testing.T) {
	t.Parallel()

	exprs := []string{
		"exe[1-10]", "exe[0001-0016],sub[01-02],login",
		"exe[1-2]-ib[0-1]", "exe1,exe01,exe001",
		"rack[1-3]node[01-04]", "10.0.1.[1-8]",
		"exe[1-5]!exe3", "exe[1-100]",
		"exe[01-02],exe3", "exe[0001-0010],exe11", "exe7,exe08,exe9",
		"exe[08-12]!exe[08-09]", "x[1-2]y[01-02],x3y3", "exe[00-01],exe0",
	}
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			first, err := nodeset.Parse(expr)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", expr, err)
			}
			folded := first.String()
			second, err := nodeset.Parse(folded)
			if err != nil {
				t.Fatalf("Parse(%q) of folded form failed: %v", folded, err)
			}
			if got := second.String(); got != folded {
				t.Errorf("folding is not stable: %q then %q", folded, got)
			}
			if !equal(first.Expand(), second.Expand()) {
				t.Errorf("folding lost hosts: %v vs %v", first.Expand(), second.Expand())
			}
		})
	}
}

// TestFoldLargeSets folds sets far larger than any cluster. Folding has to stay
// close to linear: merging a dimension one host at a time and sorting it after
// every merge turns this test from milliseconds into minutes.
func TestFoldLargeSets(t *testing.T) {
	t.Parallel()

	// A set that is no product folds in passes over its hosts, which have to
	// stay close to linear too: comparing every pair of vectors, as
	// ClusterShell does, takes minutes here.
	t.Run("a set with holes", func(t *testing.T) {
		t.Parallel()
		ns := nodeset.MustParse("rack[1-256]-exe[1-256]!rack[1-256/7]-exe[1-256/3]")
		folded := ns.String()
		if back := nodeset.MustParse(folded); back.Len() != ns.Len() || back.String() != folded {
			t.Errorf("String() = %.60q does not read back as the same set", folded)
		}
	})

	// A host named again lists the hosts of a bracketed name one by one,
	// and the fold has to find the ranges again.
	for _, tc := range []struct{ expr, again string }{
		{"exe[1-262144]", "exe1"},
		{"rack[1-256]-exe[1-256]", "rack1-exe1"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			if got := nodeset.MustParse(tc.expr + "," + tc.again).String(); got != tc.expr {
				t.Errorf("String() = %.60q, want %q", got, tc.expr)
			}
		})
	}
}

func TestAutostep(t *testing.T) {
	t.Parallel()

	ns, err := nodeset.Parse("exe[1,3,5,7,9]", nodeset.WithAutostep(3))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if got, want := ns.String(), "exe[1-9/2]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	plain, err := nodeset.Parse("exe[1,3,5,7,9]")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if got, want := plain.String(), "exe[1,3,5,7,9]"; got != want {
		t.Errorf("without autostep String() = %q, want %q", got, want)
	}

	// A progression shorter than the threshold, or one whose values read
	// differently at the width of its first, is left as it is. A threshold
	// of two makes a step of any two values, the last two of a list
	// included, as ClusterShell does.
	for _, tc := range []struct {
		expr     string
		autostep int
		want     string
	}{
		{"exe[1,3,5,7,9,20,22]", 3, "exe[1-9/2,20,22]"},
		{"exe[1,03,5,7]", 3, "exe[1,03,5,7]"},
		{"exe[1,3,5,7,9,20,22]", 2, "exe[1-9/2,20-22/2]"},
		{"node[154,176]", 2, "node[154-176/22]"},
		{"node[154,176]", 3, "node[154,176]"},
		{"exe[1,03]", 2, "exe[1,03]"},
		{"exe7", 2, "exe7"},
		// A value next to a progression is left on its own, and so is a
		// value of another width, as ClusterShell leaves them.
		{"exe[1,3,5,6]", 3, "exe[1-5/2,6]"},
		{"exe[1,3,5,6,10]", 3, "exe[1-5/2,6,10]"},
		{"exe[1,3,5,007]", 3, "exe[1-5/2,007]"},
	} {
		if got := nodeset.MustParse(tc.expr, nodeset.WithAutostep(tc.autostep)).String(); got != tc.want {
			t.Errorf("String() of %q with autostep %d = %q, want %q", tc.expr, tc.autostep, got, tc.want)
		}
	}

	// A set built with New folds the same way.
	built := nodeset.New(nodeset.WithAutostep(3))
	if err := built.Add("exe[1,3,5]"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if got, want := built.String(), "exe[1-5/2]"; got != want {
		t.Errorf("String() of a set from New = %q, want %q", got, want)
	}
}

func TestSetOperations(t *testing.T) {
	t.Parallel()

	a := nodeset.MustParse("exe[1-5]")
	b := nodeset.MustParse("exe[4-8]")

	cases := []struct {
		name string
		got  *nodeset.NodeSet
		want string
	}{
		{"union", a.Union(b), "exe[1-8]"},
		{"intersection", a.Intersection(b), "exe[4-5]"},
		{"difference", a.Difference(b), "exe[1-3]"},
		{"symmetric difference", a.SymmetricDifference(b), "exe[1-3,6-8]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got.String(); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
	if got, want := a.String(), "exe[1-5]"; got != want {
		t.Errorf("the operands were modified: %q, want %q", got, want)
	}

	// Hosts of patterns only one operand holds.
	c := nodeset.MustParse("sub[1-2],exe3")
	mixed := []struct {
		name string
		got  *nodeset.NodeSet
		want string
	}{
		{"union", a.Union(c), "exe[1-5],sub[1-2]"},
		{"intersection", nodeset.MustParse("exe[1-5],login").Intersection(b), "exe[4-5]"},
		{"difference", a.Difference(nodeset.MustParse("sub1,exe9")), "exe[1-5]"},
		{"symmetric difference", a.SymmetricDifference(c), "exe[1-2,4-5],sub[1-2]"},
	}
	for _, tc := range mixed {
		if got := tc.got.String(); got != tc.want {
			t.Errorf("%s with another pattern = %q, want %q", tc.name, got, tc.want)
		}
		// The result is a set of its own: changing it leaves the operands
		// as they were.
		if err := tc.got.Add("sub3"); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := c.String(), "exe3,sub[1-2]"; got != want {
		t.Errorf("the operands were modified: %q, want %q", got, want)
	}
}

func TestOperatorsEvaluateLeftToRight(t *testing.T) {
	t.Parallel()

	// (exe[1-10] minus exe[1-5]) intersected with exe[1-7] is exe[6-7];
	// grouping the operators any other way gives a different answer.
	ns := nodeset.MustParse("exe[1-10]!exe[1-5]&exe[1-7]")
	if got, want := ns.String(), "exe[6-7]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestGroups(t *testing.T) {
	t.Parallel()

	res := &nodeset.MapResolver{
		Groups: map[string]map[string]string{
			"local": {
				"compute": "exe[1-4]",
				"submit":  "sub[1-2]",
				"all":     "@compute,@submit",
				"empty":   "",
			},
			"slurm": {
				"idle":    "exe[3-4]",
				"drained": "exe4",
				// A bare reference inside a group of a named source means
				// a group of that source, as it does in ClusterShell.
				"up":        "@idle!@drained",
				"local":     "@local:submit,@idle",
				"elsewhere": "@local:all",
				"every":     "@local:*",
			},
		},
		Default: "local",
	}

	tests := []struct {
		expr string
		want string
	}{
		{"@compute", "exe[1-4]"},
		{"@local:compute", "exe[1-4]"},
		{"@slurm:idle", "exe[3-4]"},
		{"@all", "exe[1-4],sub[1-2]"},
		{"@compute!@slurm:idle", "exe[1-2]"},
		{"@compute,login", "exe[1-4],login"},
		{"@empty", ""},
		{"@*", "exe[1-4],sub[1-2]"},
		{"@slurm:up", "exe3"},
		{"@slurm:local", "exe[3-4],sub[1-2]"},
		{"@slurm:elsewhere", "exe[1-4],sub[1-2]"},
		{"@slurm:every", "exe[1-4],sub[1-2]"},
		// A group the table does not hold names no hosts, as in a static
		// source of ClusterShell, and so does a reference without a name.
		{"@missing", ""},
		{"@slurm:missing,exe9", "exe9"},
		{"@", ""},
		{"@local:", ""},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			ns, err := nodeset.ParseWith(tc.expr, res)
			if err != nil {
				t.Fatalf("ParseWith(%q) failed: %v", tc.expr, err)
			}
			if got := ns.String(); got != tc.want {
				t.Errorf("ParseWith(%q) = %q, want %q", tc.expr, got, tc.want)
			}
		})
	}
}

func TestGroupErrors(t *testing.T) {
	t.Parallel()

	if _, err := nodeset.Parse("@compute"); err == nil {
		t.Error("Parse of a group without a resolver should fail")
	}

	cyclic := nodeset.NewMapResolver("local", map[string]string{"loop": "@loop"})
	if _, err := nodeset.ParseWith("@loop", cyclic); err == nil {
		t.Error("a group cycle should be reported")
	}

	res := nodeset.NewMapResolver("local", map[string]string{"compute": "exe[1-2]"})
	for _, expr := range []string{"@other:compute", "@other:*"} {
		if _, err := nodeset.ParseWith(expr, res); err == nil {
			t.Errorf("ParseWith(%q) should fail", expr)
		}
	}

	// A resolver for which an unknown group is an error keeps it one.
	if _, err := nodeset.ParseWith("exe1,@missing", strictResolver{}); err == nil {
		t.Error("ParseWith with a resolver that refuses an unknown group should fail")
	}
}

// strictResolver refuses every group, as a program does for which a
// mistyped group must not select nothing.
type strictResolver struct{}

func (strictResolver) Resolve(_, group string) (string, error) {
	return "", fmt.Errorf("unknown group %q", group)
}

func (strictResolver) All(string) (string, error) { return "", nil }

var _ nodeset.Lister = (*nodeset.MapResolver)(nil)

func TestMapResolverLists(t *testing.T) {
	t.Parallel()

	res := &nodeset.MapResolver{
		Groups: map[string]map[string]string{
			"local": {"submit": "sub[1-2]", "compute": "exe[1-4]"},
		},
		Default: "local",
	}
	if got, want := res.DefaultSource(), "local"; got != want {
		t.Errorf("DefaultSource() = %q, want %q", got, want)
	}
	for _, source := range []string{"", "local"} {
		got, err := res.List(source)
		if err != nil {
			t.Fatalf("List(%q) failed: %v", source, err)
		}
		if want := []string{"compute", "submit"}; !equal(got, want) {
			t.Errorf("List(%q) = %v, want %v", source, got, want)
		}
	}
	if _, err := res.List("other"); err == nil {
		t.Error("List of an unknown source should fail")
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	exprs := []string{
		"exe[1-", "exe1]", "exe[]", "exe[5-1]", "exe[1-2/0]",
		"exe[a-b]", "exe[1-b]", "exe[1-2/x]", "exe[3/2]", "exe[1,,2]",
		// A range without its last bound is what exe[1-$N] leaves behind
		// when N is empty, and reading it as exe1 would select one host
		// where the command meant several.
		"exe[1-]", "exe[1-,5]", "exe[1-/2]",
		"exe[1-100000000]",
		// A number is at most eighteen digits long, written bare or in
		// brackets.
		"exe1234567890123456789", "exe[1-2]x1234567890123456789",
		// A step is a plain decimal number like a bound, so it can neither
		// carry a sign nor be large enough to wrap around.
		"exe[1-9/+2]", "exe[5-999999999999999999/9223372036854775807]",
		// Adjacent numeric parts cannot be told apart once expanded.
		"exe0[0,10]", "exe[1-2][3-4]", "[1-2]0",
		// A last bound padded differently from the first would be shown
		// under a name it was not given.
		"exe[1-010]", "exe[001-10]", "exe[01-005]",
		// A set operator needs an operand on each side. A dangling one is
		// what an empty command substitution leaves behind, and dropping it
		// would select every host of the left operand.
		"exe[1-3]&", "exe[1-3]!", "exe[1-3]^", "exe[1-3]! ",
		"exe[1-3]!,exe2", "exe[1-3]&&exe2", "exe[1-3]&!exe2", "exe[1-3],&exe2",
		"&exe1", "!exe1", " ^exe1",
		// A host name never begins with a dash; ssh would read it as an
		// option.
		"-oProxyCommand=x", "exe1 -exe2", "exe1,-exe[1-2]", "-exe1 exe2",
	}
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			if _, err := nodeset.Parse(expr); err == nil {
				t.Errorf("Parse(%q) should fail", expr)
			}
		})
	}
}

func TestHostlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expr string
		want string
	}{
		{"exe[1-4]", "exe[1-4]"},
		{"exe[1-2],sub1", "exe[1-2],sub1"},
		// Every name has one bracketed range at most, folded along the
		// dimension that gives the fewest names, the last one on a tie.
		{"exe[1-2]-ib[0-1]", "exe1-ib[0-1],exe2-ib[0-1]"},
		{"r[01-02]n[001-100]", "r01n[001-100],r02n[001-100]"},
		// Names that begin alike are ordered by the numbers that follow.
		{"a1b2c2,a1b1c1", "a1b1c1,a1b2c2"},
		{"exe[0001-0100].mgmt.dc2.example.org", "exe[0001-0100].mgmt.dc2.example.org"},
		// Slurm and FreeIPMI read no steps, whatever the set was parsed with.
		{"exe[1,3,5,7]", "exe[1,3,5,7]"},
		{"exe[0001-0010],exe11", "exe[0001-0010,11]"},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			ns := nodeset.MustParse(tc.expr, nodeset.WithAutostep(2))
			if got := ns.Hostlist(); got != tc.want {
				t.Errorf("Hostlist() = %q, want %q", got, tc.want)
			}
			back := nodeset.MustParse(ns.Hostlist())
			if !equal(back.Expand(), ns.Expand()) {
				t.Errorf("Hostlist() %q names %v, want %v", ns.Hostlist(), back.Expand(), ns.Expand())
			}
		})
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()

	ns := nodeset.MustParse("exe[1-7]")
	chunks := ns.Split(3)
	if len(chunks) != 3 {
		t.Fatalf("Split(3) returned %d chunks, want 3", len(chunks))
	}
	want := []string{"exe[1-3]", "exe[4-5]", "exe[6-7]"}
	total := 0
	for i, c := range chunks {
		if got := c.String(); got != want[i] {
			t.Errorf("chunk %d = %q, want %q", i, got, want[i])
		}
		total += c.Len()
	}
	if total != ns.Len() {
		t.Errorf("chunks hold %d hosts, want %d", total, ns.Len())
	}
	if got := ns.Split(0); got != nil {
		t.Errorf("Split(0) = %v, want nil", got)
	}
	if got := nodeset.New().Split(2); got != nil {
		t.Errorf("splitting an empty set = %v, want nil", got)
	}
	if got := len(ns.Split(100)); got != ns.Len() {
		t.Errorf("Split(100) returned %d chunks, want %d", got, ns.Len())
	}
}

func TestAddAndContains(t *testing.T) {
	t.Parallel()

	ns := nodeset.New()
	if err := ns.Add("exe[1-2]"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := ns.Add("sub1"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if !ns.Contains("exe1") || !ns.Contains("sub1") {
		t.Errorf("Contains missed a member of %q", ns)
	}
	if !ns.Contains("exe01") {
		t.Error("exe01 and exe1 name the same host, so Contains must match")
	}
	if !ns.Contains("exe[2]") {
		t.Error("exe[2] names one host of the set, so Contains must match")
	}
	for _, name := range []string{"exe9", "sub9", "login", "not a host[", "exe1]", "exe[1-2]", "-exe1", "exe1234567890123456789", ""} {
		if ns.Contains(name) {
			t.Errorf("Contains(%q) matched a host that is not a member", name)
		}
	}
	if err := ns.Add("exe["); err == nil {
		t.Error("Add of a malformed expression should fail")
	}
	if ns.IsEmpty() {
		t.Error("IsEmpty on a populated set")
	}
}

func TestMustParsePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("MustParse should panic on a malformed expression")
		}
	}()
	nodeset.MustParse("exe[")
}

func TestLargeSetIsRejected(t *testing.T) {
	t.Parallel()

	if _, err := nodeset.Parse("exe[1-2000]x[1-2000]"); err == nil {
		t.Error("a set beyond the element cap should be rejected")
	} else if !strings.Contains(err.Error(), "hosts") {
		t.Errorf("error = %v, want it to mention the host count", err)
	}
}

// TestExpressionLimits lowers the limits, so it must not run in parallel.
func TestExpressionLimits(t *testing.T) {
	nodeset.LowerLimits(t, 100)

	for _, expr := range []string{
		"exe[1-100]", "exe[1-50,51-100]", "exe[1-10]-ib[1-10]",
		"exe[1-50],exe[1-50]", "exe[1-50]!exe[1-25],sub[1-25]",
	} {
		if _, err := nodeset.Parse(expr); err != nil {
			t.Errorf("Parse(%q) failed within the limits: %v", expr, err)
		}
	}

	rejected := []struct{ expr, mentions string }{
		{"exe[1-101]", "elements"},
		// A range written as several parts is capped as a whole.
		{"exe[1-60,61-120]", "elements"},
		{"exe[1-10]-ib[1-11]", "hosts"},
		// So is an expression made of several terms, at every step.
		{"exe[1-60],sub[1-60]", "hosts"},
		{"exe[1-60] sub[1-60]!sub[1-60]", "hosts"},
		// Every term counts, however small the result stays, so the work
		// an expression costs is bounded as well as its size.
		{"exe[1-100]!exe[1-100],exe1", "together"},
		{"a[1-60]!a[1-60],b1", "together"},
		{"exe[1-100],login", "together"},
		{"exe[1-100]!login", "together"},
		// A pattern is weighed one dimension at a time, before the next is
		// expanded.
		{"a[1-2]b[1-2]c[1-2]d[1-2]e[1-2]f[1-2]g[1-2]h[1-2]", "hosts"},
	}
	for _, tc := range rejected {
		_, err := nodeset.Parse(tc.expr)
		if err == nil {
			t.Errorf("Parse(%q) should exceed the limits", tc.expr)
			continue
		}
		if !strings.Contains(err.Error(), tc.mentions) {
			t.Errorf("Parse(%q) error = %v, want it to mention %s", tc.expr, err, tc.mentions)
		}
	}

	res := nodeset.NewMapResolver("local", map[string]string{
		"big": "exe[1-60],sub[1-60]", "half": "exe[1-60]",
	})
	if _, err := nodeset.ParseWith("@big", res); err == nil {
		t.Error("a group beyond the limits should be rejected")
	}
	// The terms of a group count towards the expression that refers to it.
	if _, err := nodeset.ParseWith("@half!@half,@half", res); err == nil {
		t.Error("groups beyond the limits together should be rejected")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestOversizedExpressionsAreRefusedEarly runs the two expressions that used
// to be refused, or accepted, only after gigabytes had been allocated: a term
// of two hundred full brackets, and a difference that cancels out repeated.
// Neither may cost more than the largest set an expression may name. It
// measures allocation across the whole process, so it must not run in
// parallel.
func TestOversizedExpressionsAreRefusedEarly(t *testing.T) {
	full := "[0-1048575]"
	allocated := func(expr string) (uint64, error) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := nodeset.Parse(expr)
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc, err
	}

	largest, err := allocated("a" + full)
	if err != nil {
		t.Fatalf("the largest set was refused: %v", err)
	}
	for _, expr := range []string{
		"a" + strings.Repeat(full+"a", 200),
		strings.Repeat("a"+full+"!a"+full+",", 3),
	} {
		spent, err := allocated(expr)
		if err == nil {
			t.Errorf("Parse(%.40q...) should be refused", expr)
		}
		if spent > largest+largest/2 {
			t.Errorf("Parse(%.40q...) allocated %d MiB before refusing, the largest set costs %d MiB",
				expr, spent>>20, largest>>20)
		}
	}
}

// TestContainsDoesNotAllocate holds a lookup of a name without brackets to
// no allocation, in a set kept as ranges and in one whose hosts are listed.
func TestContainsDoesNotAllocate(t *testing.T) {
	for _, expr := range []string{"r[1-20]n[001-128]", "r1n001,r3n017,r20n128"} {
		ns := nodeset.MustParse(expr)
		allocs := testing.AllocsPerRun(100, func() {
			ns.Contains("r3n017")
			ns.Contains("r21n1")
			ns.Contains("login1")
		})
		if allocs != 0 {
			t.Errorf("Contains in %q allocated %v times per run, want 0", expr, allocs)
		}
	}
}

// TestCanonicalReturnsAHeldName covers a set that holds hosts of one pattern
// written with different widths. Canonical used to render every member at one
// width per pattern, so it answered with names the set was never given.
func TestCanonicalReturnsAHeldName(t *testing.T) {
	t.Parallel()

	ns := nodeset.MustParse("exe[0001-0010],exe11,lab1")
	tests := []struct{ ask, want string }{
		{"exe1", "exe0001"},
		{"exe0001", "exe0001"},
		{"exe11", "exe11"},
		{"exe0011", "exe11"},
		{"lab01", "lab1"},
	}
	for _, tc := range tests {
		got, ok := ns.Canonical(tc.ask)
		if !ok || got != tc.want {
			t.Errorf("Canonical(%q) = %q, %v, want %q", tc.ask, got, ok, tc.want)
		}
	}
	for _, name := range []string{"exe12", "exe[1-2]", "-exe1", ""} {
		if got, ok := ns.Canonical(name); ok {
			t.Errorf("Canonical(%q) = %q, want no match", name, got)
		}
	}

	// A set kept as a product of ranges answers from the ranges.
	p := nodeset.MustParse("r[1-2]n[001-128]")
	for ask, want := range map[string]string{"r2n5": "r2n005", "r[1]n[128]": "r1n128"} {
		if got, ok := p.Canonical(ask); !ok || got != want {
			t.Errorf("Canonical(%q) = %q, %v, want %q", ask, got, ok, want)
		}
	}
	for _, name := range []string{"r3n1", "r1n129", "r1n1-"} {
		if got, ok := p.Canonical(name); ok {
			t.Errorf("Canonical(%q) = %q, want no match", name, got)
		}
	}

	// Combining sets keeps the spelling already held.
	u := nodeset.MustParse("lab1").Union(nodeset.MustParse("lab01,lab2"))
	if got, want := u.String(), "lab[1-2]"; got != want {
		t.Errorf("union = %q, want %q", got, want)
	}
	for _, chunk := range ns.Split(2) {
		for _, name := range chunk.Expand() {
			if !contains(ns.Expand(), name) {
				t.Errorf("Split renamed a host to %q", name)
			}
		}
	}
}

func contains(names []string, name string) bool {
	return slices.Contains(names, name)
}

// TestConcurrentReads reads one set from several goroutines at once, which
// the race detector checks: a set folds once and keeps the fold, and the
// readers that fold it first do so at the same time.
func TestConcurrentReads(t *testing.T) {
	t.Parallel()

	ns := nodeset.MustParse("exe[1-100]!exe50,rack[1-4]node[1-8]!rack2node3,login")
	want := ns.Clone()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := ns.String(); got != want.String() {
				t.Errorf("String() = %q, want %q", got, want.String())
			}
			if got := ns.Expand(); !equal(got, want.Expand()) {
				t.Errorf("Expand() = %v, want %v", got, want.Expand())
			}
			if !ns.Contains("rack1node1") || ns.Len() != want.Len() {
				t.Error("a concurrent read saw another set")
			}
		})
	}
	wg.Wait()
}
