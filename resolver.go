// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// maxGroupDepth bounds how deeply groups may reference other groups, which
// also breaks reference cycles.
const maxGroupDepth = 16

// Resolver turns a group reference into a node set expression.
//
// A reference is written @group, or @source:group when several group sources
// are configured. The expression a resolver returns is parsed in turn, so a
// group may refer to other groups.
//
// The source is passed on as written, and is empty for a bare @group: which
// source that means, the default one or a search of several, is the
// resolver's to decide. A bare @group inside a group of a named source is
// passed on with that source, as ClusterShell resolves it.
//
// A resolver whose every lookup is a round trip can also implement
// BatchResolver, and be asked for the groups of an expression at once.
type Resolver interface {
	// Resolve returns the expression a group names; an empty group returns
	// an empty expression. What an unknown group means is the resolver's to
	// decide: an error, for a program that must not select nothing by
	// mistake, or no hosts, as ClusterShell's static sources and MapResolver
	// answer. The name is empty for a reference written @ or @source:.
	Resolve(source, group string) (string, error)
	// All returns the expression naming every host a source knows. It
	// answers the reference @source:*, or @* for an empty source.
	//
	// The expression is evaluated as one, left to right, so a resolver
	// that joins the expressions of several groups into it has to keep the
	// operators of each group to that group, as MapResolver does.
	All(source string) (string, error)
}

// Lister is implemented by a Resolver that can also say which groups it
// offers, for a program that lists them or completes a group name. Parsing
// never needs it.
type Lister interface {
	// List returns the group names a source offers. An empty source means
	// the default one.
	List(source string) ([]string, error)
	// DefaultSource names the source used when a reference names none.
	DefaultSource() string
}

// BatchResolver is implemented by a Resolver that can look up several groups
// at once, as one backed by a command, a directory or a remote service would
// rather do than make one round trip after the other.
//
// Before ParseWith evaluates an expression, the one it is given or one a
// group answers with, it asks ResolveBatch for the groups the expression
// refers to that it has not looked up yet, when there are at least two. The
// evaluation takes each group's answer from there, and asks Resolve for a
// group that got none. Within one call of ParseWith, a group that has been
// answered, failure included, is not asked for again; keeping answers from
// one call to the next is the resolver's to do. All is asked as before.
//
// An expression names the same hosts, and fails with the same error, as it
// does through Resolve alone: an answer's error is reported when the
// evaluation reaches that group. An expression that fails may have had
// groups looked up that its evaluation did not reach. Which groups are asked
// for together is not part of the API, any more than speed is.
//
// The package starts no goroutines: whether the groups are looked up in
// parallel, in one request or from a cache is the resolver's to decide.
type BatchResolver interface {
	Resolver
	// ResolveBatch returns, in the order of refs, what Resolve returns for
	// each. A reference has the source Resolve would be given for it, refs
	// holds none twice, and holds neither @* nor @source:*, which All
	// answers. A shorter slice, or nil, leaves the groups without an
	// answer to Resolve, as for a lookup that was cut short.
	ResolveBatch(refs []GroupRef) []GroupAnswer
}

// GroupRef is a group reference: @Source:Group, or @Group when Source is
// empty.
type GroupRef struct {
	Source, Group string
}

// GroupAnswer is what a resolver answers for a group: the expression it
// names, or the error that stopped the lookup.
type GroupAnswer struct {
	Expr string
	Err  error
}

// groupRef reads a reference written without its @: the source is what
// comes before the first colon, if there is one.
func groupRef(ref string) GroupRef {
	if source, group, ok := strings.Cut(ref, ":"); ok {
		return GroupRef{Source: source, Group: group}
	}
	return GroupRef{Group: ref}
}

// resolveGroup evaluates one @reference.
func resolveGroup(ref string, res Resolver, depth int, b *budget) (*NodeSet, error) {
	if res == nil {
		return nil, fmt.Errorf("group @%s cannot be resolved: no group source is configured", ref)
	}
	// The source is passed on exactly as written, empty included, so that a
	// resolver backed by several sources can search them all for a bare
	// @group reference rather than being limited to its default.
	r := groupRef(ref)
	source, group := r.Source, r.Group

	var (
		expr string
		err  error
	)
	if group == "*" {
		expr, err = res.All(source)
	} else {
		expr, err = res.Resolve(source, group)
	}
	if err != nil {
		return nil, fmt.Errorf("group @%s: %w", ref, err)
	}
	if strings.TrimSpace(expr) == "" {
		return New(), nil
	}
	if source != "" {
		if in, ok := res.(inSource); ok {
			res = in.Resolver
		}
		res = inSource{Resolver: res, source: source}
	}
	return parseExpression(expr, res, depth+1, b)
}

// inSource resolves the bare references inside a group of a named source in
// that source, as ClusterShell does: a group of source ib that refers to
// @fabric means @ib:fabric.
type inSource struct {
	Resolver
	source string
}

func (r inSource) Resolve(source, group string) (string, error) {
	return r.Resolver.Resolve(cmp.Or(source, r.source), group)
}

func (r inSource) All(source string) (string, error) {
	return r.Resolver.All(cmp.Or(source, r.source))
}

// batch looks up the groups of an expression at once through a
// BatchResolver, before the expression is evaluated, and answers the
// evaluation from what it found. ParseWith makes one for each call, so that
// answers are kept for one parse.
type batch struct {
	BatchResolver
	answers map[GroupRef]GroupAnswer
}

// Resolve implements Resolver from what lookUp found, and otherwise asks the
// resolver and keeps its answer, so that a group answered is not asked for
// again.
func (bt *batch) Resolve(source, group string) (string, error) {
	ref := GroupRef{Source: source, Group: group}
	a, ok := bt.answers[ref]
	if !ok {
		a.Expr, a.Err = bt.BatchResolver.Resolve(source, group)
		bt.answers[ref] = a
	}
	return a.Expr, a.Err
}

// lookUp asks ResolveBatch for the groups expr refers to that have no answer
// yet, when there are at least two, reading the terms as the evaluation
// reads them. A bare reference is looked up in source, as inSource resolves
// it. An expression with a syntax error has only the groups before the
// error looked up, since its evaluation reaches none after it, and the
// evaluation reports the error.
func (bt *batch) lookUp(expr, source string) {
	var refs []GroupRef
	seen := make(map[GroupRef]bool)
	_ = splitTerms(expr, func(_ operator, term string) error {
		if term[0] != '@' {
			return nil
		}
		ref := groupRef(term[1:])
		ref.Source = cmp.Or(ref.Source, source)
		if _, answered := bt.answers[ref]; !answered && ref.Group != "*" && !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
		return nil
	})
	if len(refs) < 2 {
		return
	}
	// The resolver may keep or reorder the slice it is given.
	answers := bt.ResolveBatch(slices.Clone(refs))
	for i, a := range answers[:min(len(answers), len(refs))] {
		bt.answers[refs[i]] = a
	}
}

// batchOf returns the batch res looks up groups through, if it has one, and
// the source in which a bare reference is looked up.
func batchOf(res Resolver) (*batch, string) {
	source := ""
	if in, ok := res.(inSource); ok {
		res, source = in.Resolver, in.source
	}
	bt, _ := res.(*batch)
	return bt, source
}

// MapResolver resolves groups from an in-memory table, such as groups a
// program reads from its configuration, or those of a test.
type MapResolver struct {
	// Groups maps a source name to its groups.
	Groups map[string]map[string]string
	// Default names the source used when a reference names none.
	Default string
}

// NewMapResolver builds a resolver for a single source, which is also the
// default one.
func NewMapResolver(source string, groups map[string]string) *MapResolver {
	return &MapResolver{
		Groups:  map[string]map[string]string{source: groups},
		Default: source,
	}
}

// Resolve implements Resolver. An empty source means the default one. A
// group the source does not hold names no hosts, as it does in a static
// source of ClusterShell; a source the table does not hold is an error.
func (m *MapResolver) Resolve(source, group string) (string, error) {
	source = cmp.Or(source, m.Default)
	groups, ok := m.Groups[source]
	if !ok {
		return "", fmt.Errorf("unknown group source %q", source)
	}
	return groups[group], nil
}

// List implements Lister. An empty source means the default one.
func (m *MapResolver) List(source string) ([]string, error) {
	source = cmp.Or(source, m.Default)
	groups, ok := m.Groups[source]
	if !ok {
		return nil, fmt.Errorf("unknown group source %q", source)
	}
	out := make([]string, 0, len(groups))
	for name := range groups {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// All implements Resolver by unioning every group of the source, each
// evaluated on its own. A group whose expression holds an operator other
// than the union is referred to as @source:group rather than written out:
// its operator would otherwise apply to every group before it. So is a
// group whose brackets do not balance, which would otherwise take in the
// group after it. A group referred to is evaluated one level of nesting
// deeper than one written out. A group that has to be referred to is an
// error when its name is empty or *, or holds whitespace, a comma, an
// operator or a bracket, or when the name of its source holds one of those
// or a colon, since such a reference might not read back as that group.
func (m *MapResolver) All(source string) (string, error) {
	names, err := m.List(source)
	if err != nil {
		return "", err
	}
	source = cmp.Or(source, m.Default)
	groups := m.Groups[source]
	parts := make([]string, 0, len(names))
	for _, name := range names {
		part, err := unionOperand(source, name, groups[name])
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ","), nil
}

// unionOperand writes a group's expression as one operand of a union: as
// it is when it holds no operator but the union and its brackets balance,
// and otherwise as a reference to the group, which the parser evaluates on
// its own. A group is refused when its name, or its source's, might not
// read back as that one reference, since such a reference could name other
// hosts.
func unionOperand(source, group, expr string) (string, error) {
	if !strings.ContainsAny(expr, "!&^") && balanced(expr) {
		return expr, nil
	}
	if group == "" || group == "*" || !referable(group) || !referable(source) || strings.Contains(source, ":") {
		return "", fmt.Errorf("group %q of source %q holds a set operator or an unbalanced bracket, "+
			"and %q might not read back as that group, "+
			"so %q cannot evaluate it on its own", group, source, "@"+source+":"+group, "@"+source+":*")
	}
	return "@" + source + ":" + group, nil
}

// balanced reports whether every bracket of an expression is closed, as the
// parser counts them. A group whose brackets do not balance would otherwise
// take the comma after it, and the group after that, into its range.
func balanced(expr string) bool {
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '[':
			depth++
		case ']':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// referable reports whether a name is free of what the parser splits a term
// at, or trims from it: whitespace of any kind, a comma, an operator and a
// bracket. A group named "a" followed by a no-break space would be read back
// as the group "a". Some of the names it refuses would read back, such as
// one holding a pair of brackets; they are refused all the same, as
// decision 11 in doc/decisions.md says.
func referable(name string) bool {
	return !strings.ContainsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",!&^[]", r)
	})
}

// DefaultSource implements Lister.
func (m *MapResolver) DefaultSource() string { return m.Default }
