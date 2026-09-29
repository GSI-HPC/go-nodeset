// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/GSI-HPC/go-nodeset"
)

// oracleGroups are the group sources both implementations resolve groups
// from. Their hosts follow the templates below, so that a group and a term
// never write one host with two paddings. Groups refer to other groups of
// their own source bare and to those of another with the source written out.
var oracleGroups = map[string]map[string]string{
	"site": {
		"compute": "exe[0001-0120]",
		"gpu":     "gpu[01-16].hpc.example.org",
		"racks":   "rack[01-04]n[01-08]",
		"cpu":     "@compute!exe[0100-0120]",
		"login":   "login,head",
	},
	"ib": {
		"fabric": "cn[001-032]-ib[0-1]",
		"edge":   "@fabric&cn[001-004]-ib0",
		"mixed":  "@site:login,sw[1-4]p[01-48],@edge",
	},
}

// oracleRefs are the group references a generated term may be.
var oracleRefs = []string{
	"@compute", "@gpu", "@racks", "@cpu", "@login", "@site:compute",
	"@ib:fabric", "@ib:edge", "@ib:mixed", "@*", "@site:*", "@ib:*",
	"@nosuch", "@ib:nosuch", "@", "@ib:",
}

// An oracleTemplate is the shape of a host name: literals around numeric
// dimensions, each written at one width in every expression, so that no
// generated expression names one host under two paddings, which is where the
// package differs from ClusterShell on purpose (doc/language.md).
type oracleTemplate struct {
	lits   []string // one more than widths
	widths []int
}

var oracleTemplates = []oracleTemplate{
	{[]string{"exe", ""}, []int{4}},
	{[]string{"node", ""}, []int{1}},
	{[]string{"rack", "n", ""}, []int{2, 2}},
	{[]string{"cn", "-ib", ""}, []int{3, 1}},
	{[]string{"gpu", ".hpc.example.org"}, []int{2}},
	{[]string{"10.0.", ".", ""}, []int{1, 1}},
	{[]string{"sw", "p", ""}, []int{1, 2}},
	{[]string{"bmc-", "-", ".mgmt"}, []int{2, 3}},
	{[]string{"login"}, nil},
	{[]string{"head"}, nil},
}

// oracleRejected are expressions both implementations must refuse. Where
// the package refuses what ClusterShell accepts, a malformed range such as
// exe[1-2-3], doc/language.md says so.
var oracleRejected = []string{
	"exe[", "exe]", "exe[]", "exe[5-1]", "exe[1-3", "exe1-3]", "exe[a-b]",
	"exe[1-3/0]", "exe[1-3/x]", "exe[[1-2]]", "exe[1,2", "&exe1", "!exe1",
	"^exe1", "exe1&", "exe1!", "exe1^", "exe[1-3]&&exe2", "exe[1-3]!!exe2",
	"exe[1-3]&,exe2", "@nosource:compute", "@nosource:*",
}

type oracleRequest struct {
	Expr     string `json:"expr"`
	Autostep int    `json:"autostep"`
	corpus   bool
}

type oracleAnswer struct {
	Version string   `json:"version"`
	Hosts   []string `json:"hosts"`
	Folded  string   `json:"folded"`
	Error   string   `json:"error"`
}

// TestClusterShellOracle compares the package with ClusterShell itself, run
// as testdata/clustershell_oracle.py, on the lines of the corpus that name no
// divergence, on expressions both must reject, and on thousands of
// expressions generated from a fixed seed: ranges, steps, padding, several
// dimensions, domains, the four operators, nested groups from two sources,
// and autostep. For each it checks that
//
//   - both reject it, or neither does;
//   - both name the same hosts;
//   - ClusterShell reads String and Hostlist back as those hosts, and the
//     package reads ClusterShell's folded form back as those hosts;
//   - String is ClusterShell's folded form, autostep included, and Expand
//     lists the hosts in ClusterShell's order.
//
// Lines of the corpus are held to the same hosts only, since some write one
// host at two widths, which ClusterShell orders differently.
//
// It needs a Python with the ClusterShell of testdata/requirements.txt and is
// skipped unless NODESET_CLUSTERSHELL_PYTHON names it; make clustershell sets
// one up. NODESET_CLUSTERSHELL_CASES and NODESET_CLUSTERSHELL_SEED change how
// many expressions are generated, and from which seed.
func TestClusterShellOracle(t *testing.T) {
	python := os.Getenv("NODESET_CLUSTERSHELL_PYTHON")
	if python == "" {
		t.Skip("NODESET_CLUSTERSHELL_PYTHON is not set; run make clustershell")
	}
	cases := oracleEnv(t, "NODESET_CLUSTERSHELL_CASES", 5000)
	seed := oracleEnv(t, "NODESET_CLUSTERSHELL_SEED", 1)

	res := &nodeset.MapResolver{Groups: oracleGroups, Default: "site"}
	reqs := oracleCorpus(t)
	for _, expr := range oracleRejected {
		reqs = append(reqs, oracleRequest{Expr: expr})
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	for range cases {
		reqs = append(reqs, oracleGenerate(rng))
	}

	// Each request is followed by two for what the package prints for it,
	// String and Hostlist, which ClusterShell must read as the same hosts.
	type goAnswer struct {
		ns  *nodeset.NodeSet
		err error
	}
	ours := make([]goAnswer, len(reqs))
	var asked []oracleRequest
	for i, req := range reqs {
		var opts []nodeset.Option
		if req.Autostep > 0 {
			opts = append(opts, nodeset.WithAutostep(req.Autostep))
		}
		ns, err := nodeset.ParseWith(req.Expr, res, opts...)
		ours[i] = goAnswer{ns, err}
		var folded, hostlist string
		if err == nil {
			folded, hostlist = ns.String(), ns.Hostlist()
		}
		asked = append(asked, req, oracleRequest{Expr: folded}, oracleRequest{Expr: hostlist})
	}

	version, theirs := oracleAsk(t, python, res, asked)
	t.Logf("compared %d expressions with ClusterShell %s", len(reqs), version)

	failures := 0
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf(format, args...)
		if failures++; failures == 50 {
			t.Fatal("stopping after 50 differences")
		}
	}
	for i, req := range reqs {
		our, their := ours[i], theirs[3*i]
		switch {
		case our.err != nil && their.Error != "":
			continue
		case our.err != nil:
			fail("%q: nodeset rejects it (%v), ClusterShell gives %s", req.Expr, our.err, their.Folded)
			continue
		case their.Error != "":
			fail("%q: nodeset gives %s, ClusterShell rejects it (%s)", req.Expr, our.ns, their.Error)
			continue
		}

		hosts := our.ns.Expand()
		want := slices.Sorted(slices.Values(their.Hosts))
		if !slices.Equal(slices.Sorted(slices.Values(hosts)), want) {
			fail("%q: nodeset names %v, ClusterShell %v", req.Expr, oracleShort(hosts), oracleShort(their.Hosts))
			continue
		}
		for j, form := range []string{our.ns.String(), our.ns.Hostlist()} {
			back := theirs[3*i+1+j]
			if back.Error != "" || !slices.Equal(slices.Sorted(slices.Values(back.Hosts)), want) {
				fail("%q: ClusterShell reads %q as %v (%s)", req.Expr, form, oracleShort(back.Hosts), back.Error)
			}
		}
		if back, err := nodeset.Parse(their.Folded); err != nil || !slices.Equal(slices.Sorted(slices.Values(back.Expand())), want) {
			fail("%q: nodeset does not read ClusterShell's %q as the same hosts (%v)", req.Expr, their.Folded, err)
		}

		if req.corpus {
			continue
		}
		if got := our.ns.String(); got != their.Folded {
			fail("%q (autostep %d): nodeset folds to %q, ClusterShell to %q", req.Expr, req.Autostep, got, their.Folded)
		}
		if !slices.Equal(hosts, their.Hosts) {
			fail("%q: nodeset lists %v, ClusterShell %v", req.Expr, oracleShort(hosts), oracleShort(their.Hosts))
		}
	}
}

// oracleAsk runs the oracle script once over every request and returns
// ClusterShell's version and its answers, one per request.
func oracleAsk(t *testing.T, python string, res *nodeset.MapResolver, reqs []oracleRequest) (string, []oracleAnswer) {
	t.Helper()
	config := map[string]any{"default": res.Default, "sources": res.Groups}
	all := map[string]string{}
	for name := range res.Groups {
		expr, err := res.All(name)
		if err != nil {
			t.Fatal(err)
		}
		all[name] = expr
	}
	config["all"] = all

	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	if err := enc.Encode(config); err != nil {
		t.Fatal(err)
	}
	for _, req := range reqs {
		if err := enc.Encode(req); err != nil {
			t.Fatal(err)
		}
	}

	var stderr bytes.Buffer
	cmd := exec.Command(python, "testdata/clustershell_oracle.py")
	cmd.Stdin = &in
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s testdata/clustershell_oracle.py: %v\n%s", python, err, stderr.String())
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(nil, 1<<26)
	var answers []oracleAnswer
	for scanner.Scan() {
		var a oracleAnswer
		if err := json.Unmarshal(scanner.Bytes(), &a); err != nil {
			t.Fatalf("oracle answer %q: %v", scanner.Text(), err)
		}
		answers = append(answers, a)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(answers) != len(reqs)+1 {
		t.Fatalf("the oracle gave %d answers to %d requests", len(answers)-1, len(reqs))
	}
	return answers[0].Version, answers[1:]
}

// oracleCorpus returns the expressions of the corpus for which the package
// and ClusterShell are meant to agree: the lines without a divergence.
func oracleCorpus(t *testing.T) []oracleRequest {
	t.Helper()
	data, err := os.ReadFile("testdata/clustershell.txt")
	if err != nil {
		t.Fatal(err)
	}
	var reqs []oracleRequest
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if cols := strings.Split(line, "\t"); len(cols) == 2 {
			reqs = append(reqs, oracleRequest{Expr: cols[0], corpus: true})
		}
	}
	return reqs
}

// oracleGenerate builds one expression of one to four terms, each a host
// pattern or a group reference, joined by the four operators.
func oracleGenerate(rng *rand.Rand) oracleRequest {
	var b strings.Builder
	for i := range 1 + rng.IntN(4) {
		if i > 0 {
			b.WriteString(oraclePick(rng, []string{",", ",", ",", "!", "!", "&", "^"}))
		}
		if rng.IntN(7) == 0 {
			b.WriteString(oraclePick(rng, oracleRefs))
		} else {
			b.WriteString(oraclePattern(rng))
		}
	}
	req := oracleRequest{Expr: b.String()}
	if rng.IntN(4) == 0 {
		req.Autostep = 2 + rng.IntN(3)
	}
	return req
}

// oraclePattern writes a host pattern after one of the templates, each
// dimension as a single number or as a bracketed list of numbers, ranges and
// stepped ranges, at the template's width.
func oraclePattern(rng *rand.Rand) string {
	tpl := oracleTemplates[rng.IntN(len(oracleTemplates))]
	var b strings.Builder
	b.WriteString(tpl.lits[0])
	for d, width := range tpl.widths {
		top := 199
		if width == 2 {
			top = 99
		}
		num := func(n int) string { return fmt.Sprintf("%0*d", width, n) }
		if rng.IntN(3) == 0 {
			b.WriteString(num(rng.IntN(top + 1)))
		} else {
			items := make([]string, 1+rng.IntN(3))
			for j := range items {
				lo := rng.IntN(top + 1)
				hi := min(top, lo+rng.IntN(21))
				switch rng.IntN(4) {
				case 0:
					items[j] = num(lo)
				case 1:
					items[j] = num(lo) + "-" + num(hi) + "/" + strconv.Itoa(2+rng.IntN(3))
				default:
					items[j] = num(lo) + "-" + num(hi)
				}
			}
			b.WriteString("[" + strings.Join(items, ",") + "]")
		}
		b.WriteString(tpl.lits[d+1])
	}
	return b.String()
}

func oraclePick(rng *rand.Rand, from []string) string { return from[rng.IntN(len(from))] }

// oracleEnv reads a positive number from the environment, or returns def.
func oracleEnv(t *testing.T, name string, def int) int {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		t.Fatalf("%s=%q is not a positive number", name, v)
	}
	return n
}

// oracleShort keeps a failure message readable when a set is large.
func oracleShort(hosts []string) []string {
	if len(hosts) > 12 {
		return append(slices.Clip(hosts[:12]), fmt.Sprintf("... (%d hosts)", len(hosts)))
	}
	return hosts
}
