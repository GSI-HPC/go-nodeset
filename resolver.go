// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"fmt"
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

// resolveGroup evaluates one @reference.
func resolveGroup(ref string, res Resolver, depth int, b *budget) (*NodeSet, error) {
	if res == nil {
		return nil, fmt.Errorf("group @%s cannot be resolved: no group source is configured", ref)
	}
	// The source is passed on exactly as written, empty included, so that a
	// resolver backed by several sources can search them all for a bare
	// @group reference rather than being limited to its default.
	source, group := "", ref
	if before, after, ok := strings.Cut(ref, ":"); ok {
		source, group = before, after
	}

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
