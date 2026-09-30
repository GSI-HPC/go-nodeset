// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"fmt"
	"sort"
	"strings"
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

// All implements Resolver by unioning every group of the source.
func (m *MapResolver) All(source string) (string, error) {
	names, err := m.List(source)
	if err != nil {
		return "", err
	}
	groups := m.Groups[cmp.Or(source, m.Default)]
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, groups[name])
	}
	return strings.Join(parts, ","), nil
}

// DefaultSource implements Lister.
func (m *MapResolver) DefaultSource() string { return m.Default }
