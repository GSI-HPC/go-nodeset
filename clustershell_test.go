// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset_test

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GSI-HPC/go-nodeset"
)

// divergences are the places where this package decides otherwise than
// ClusterShell on purpose. The language reference in doc/ lists each of them.
var divergences = map[string]bool{
	"padding identity":       true,
	"adjacent numeric parts": true,
	"padding mismatch":       true,
	"whitespace":             true,
	"empty operand":          true,
	"leading dash":           true,
	"malformed range":        true,
}

// TestClusterShellCorpus runs every expression of testdata/clustershell.txt
// and compares the answer with the one ClusterShell gave. Where a line names a
// divergence the answers must differ, so that a divergence which goes away is
// noticed and taken out of the documentation too.
func TestClusterShellCorpus(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/clustershell.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	lines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 || len(cols) > 3 {
			t.Fatalf("malformed corpus line %q", line)
		}
		expr, want, tag := cols[0], cols[1], ""
		if len(cols) == 3 {
			tag = cols[2]
			if !divergences[tag] {
				t.Fatalf("line %q names an undocumented divergence %q", line, tag)
			}
		}
		lines++

		got := answer(expr)
		switch {
		case tag == "" && got != sortWords(want):
			t.Errorf("%q: nodeset gives %q, ClusterShell %q", expr, got, want)
		case tag != "" && got == sortWords(want):
			t.Errorf("%q: nodeset now agrees with ClusterShell (%q); the %s divergence is gone", expr, got, tag)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines == 0 {
		t.Fatal("the corpus is empty")
	}
}

// divergenceRow matches a row of the table of divergences in
// doc/language.md, whose last cell ends with the divergence in brackets.
var divergenceRow = regexp.MustCompile(`\(([a-z ]+)\) \|$`)

// TestDivergencesDocumented checks the table of divergences in
// doc/language.md against divergences, both ways: the table names each
// divergence the corpus may mark a line with, and no other.
func TestDivergencesDocumented(t *testing.T) {
	t.Parallel()

	doc, err := os.ReadFile("doc/language.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(doc), "\n## Where this differs from ClusterShell\n")
	if !ok {
		t.Fatal("doc/language.md has no section on where the package differs from ClusterShell")
	}
	section, _, _ = strings.Cut(section, "\n## ")

	var rows []string
	for line := range strings.Lines(section) {
		if strings.HasPrefix(line, "|") {
			rows = append(rows, strings.TrimSpace(line))
		}
	}
	if len(rows) < 3 {
		t.Fatal("doc/language.md has no table of divergences")
	}

	documented := map[string]bool{}
	for _, row := range rows[2:] {
		m := divergenceRow.FindStringSubmatch(row)
		if m == nil {
			t.Errorf("doc/language.md: the row %q names no divergence", row)
			continue
		}
		if !divergences[m[1]] {
			t.Errorf("doc/language.md names the %s divergence, which the corpus test does not know", m[1])
		}
		documented[m[1]] = true
	}
	for tag := range divergences {
		if !documented[tag] {
			t.Errorf("doc/language.md does not name the %s divergence", tag)
		}
	}
}

// answer expands an expression the way the corpus records it: the host names
// in sorted order, "-" for none, or "error".
func answer(expr string) string {
	ns, err := nodeset.Parse(expr)
	if err != nil {
		return "error"
	}
	if ns.IsEmpty() {
		return "-"
	}
	return sortWords(strings.Join(ns.Expand(), " "))
}

func sortWords(s string) string {
	words := strings.Fields(s)
	sort.Strings(words)
	return strings.Join(words, " ")
}
