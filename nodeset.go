// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

// NodeSet is an unordered set of host names that renders in folded form.
// The zero value is not usable; call New or Parse.
//
// Padding is not part of a host's identity: exe1 and exe01 are one host. Each
// host keeps the spelling it was first given, and when sets are combined the
// spelling already held wins, so a set never shows a host under a name it was
// not given.
//
// A set may be read from several goroutines at once. Add changes it, and
// needs the set to itself.
type NodeSet struct {
	groups   map[string]*group // by pattern
	autostep int
}

// Option configures a set.
type Option func(*NodeSet)

// WithAutostep folds arithmetic progressions of at least n elements into
// "first-last/step" form, taking the values from left to right as
// ClusterShell's autostep does. ClusterShell disables this by default, and so
// does this package; n below 2 keeps it disabled.
func WithAutostep(n int) Option {
	return func(ns *NodeSet) { ns.autostep = n }
}

// New returns an empty set.
func New(opts ...Option) *NodeSet {
	ns := &NodeSet{groups: make(map[string]*group)}
	for _, o := range opts {
		o(ns)
	}
	return ns
}

// Parse evaluates a node set expression without resolving groups. An
// expression containing a group reference is rejected.
func Parse(expr string, opts ...Option) (*NodeSet, error) {
	return ParseWith(expr, nil, opts...)
}

// ParseWith evaluates a node set expression, resolving group references
// through res. A nil resolver rejects every group reference.
func ParseWith(expr string, res Resolver, opts ...Option) (*NodeSet, error) {
	ns, err := parseExpression(expr, res, 0, newBudget())
	if err != nil {
		return nil, err
	}
	for _, o := range opts {
		o(ns)
	}
	return ns, nil
}

// MustParse is Parse for expressions fixed at compile time; it panics on a
// parse error.
func MustParse(expr string, opts ...Option) *NodeSet {
	ns, err := Parse(expr, opts...)
	if err != nil {
		panic(err)
	}
	return ns
}

// Len reports the number of hosts in the set.
func (ns *NodeSet) Len() int {
	n := 0
	for _, g := range ns.groups {
		n += g.len()
	}
	return n
}

// IsEmpty reports whether the set names no host.
func (ns *NodeSet) IsEmpty() bool { return len(ns.groups) == 0 }

// Clone returns an independent copy.
func (ns *NodeSet) Clone() *NodeSet {
	out := &NodeSet{groups: make(map[string]*group, len(ns.groups)), autostep: ns.autostep}
	for p, g := range ns.groups {
		out.groups[p] = g.clone()
	}
	return out
}

// Add parses an expression, without resolving groups, and adds the hosts it
// names to the set.
func (ns *NodeSet) Add(expr string) error {
	other, err := parseExpression(expr, nil, 0, newBudget())
	if err != nil {
		return err
	}
	ns.merge(other, true)
	return nil
}

// Contains reports whether name is a member. Padding is ignored, so "exe01"
// and "exe1" name the same host.
func (ns *NodeSet) Contains(name string) bool {
	_, _, _, ok := ns.lookup(name)
	return ok
}

// Canonical returns the name this set holds for a host, which is always a name
// the set was given.
//
// It differs from the name asked about when the two were written with
// different padding: a set holding exe0001 answers "exe0001" when asked about
// "exe1", because padding is not part of a host's identity and both name one
// host. A set holding exe0001 and exe11 answers "exe11" for exe11.
func (ns *NodeSet) Canonical(name string) (string, bool) {
	pattern, vals, pads, ok := ns.lookup(name)
	if !ok {
		return "", false
	}
	return string(appendName(nil, pattern, vals, pads)), true
}

// lookup finds the member a single host name refers to: its pattern, its
// values, and the widths the set holds them with.
func (ns *NodeSet) lookup(name string) (string, []int, []int, bool) {
	one, err := parseProduct(name, newBudget())
	if err != nil || one.len() != 1 {
		return "", nil, nil, false
	}
	g, ok := ns.groups[one.pattern]
	if !ok {
		return "", nil, nil, false
	}
	vals := make([]int, one.width)
	for d, rs := range one.dims {
		vals[d] = rs.values[0]
	}
	pads, ok := g.find(vals)
	return one.pattern, vals, pads, ok
}

// Expand returns the host names in the order ClusterShell lists them: by
// pattern first; within a pattern with one number, in numeric order; within
// one with several, vector by vector as String folds them, each with its last
// number varying fastest.
func (ns *NodeSet) Expand() []string {
	out := make([]string, 0, ns.Len())
	var buf []byte
	for _, p := range sortedPatterns(ns.groups) {
		ns.groups[p].inOrder(func(vals, pads []int) {
			buf = appendName(buf[:0], p, vals, pads)
			out = append(out, string(buf))
		})
	}
	return out
}

// String renders the set in folded form, which Parse reads back as the same
// set, every host spelled as before. An empty set renders as the empty string.
func (ns *NodeSet) String() string { return ns.fold() }

// Union returns the hosts in either set.
func (ns *NodeSet) Union(other *NodeSet) *NodeSet {
	out := ns.Clone()
	out.merge(other, false)
	return out
}

// Intersection returns the hosts in both sets.
func (ns *NodeSet) Intersection(other *NodeSet) *NodeSet {
	out := ns.Clone()
	out.intersect(other)
	return out
}

// Difference returns the hosts of ns that are not in other.
func (ns *NodeSet) Difference(other *NodeSet) *NodeSet {
	out := ns.Clone()
	out.subtract(other)
	return out
}

// SymmetricDifference returns the hosts in exactly one of the two sets.
func (ns *NodeSet) SymmetricDifference(other *NodeSet) *NodeSet {
	out := ns.Clone()
	out.symmetricDifference(other, false)
	return out
}

// Split partitions the set into at most n chunks of near equal size, in
// expansion order. It returns nil for n below one.
func (ns *NodeSet) Split(n int) []*NodeSet {
	total := ns.Len()
	if n < 1 || total == 0 {
		return nil
	}
	n = min(n, total)
	out := make([]*NodeSet, 0, n)
	size, rest := total/n, total%n
	var chunk *NodeSet
	left := 0
	for _, p := range sortedPatterns(ns.groups) {
		width := ns.groups[p].width
		ns.groups[p].inOrder(func(vals, pads []int) {
			if left == 0 {
				left = size
				if len(out) < rest {
					left++
				}
				chunk = &NodeSet{groups: make(map[string]*group), autostep: ns.autostep}
				out = append(out, chunk)
			}
			chunk.listed(p, width).add(vals, pads)
			left--
		})
	}
	return out
}

// listed returns the group of a pattern, made or turned into a list of hosts
// so that hosts can be added to it.
func (ns *NodeSet) listed(pattern string, width int) *group {
	g, ok := ns.groups[pattern]
	if !ok {
		g = &group{pattern: pattern, width: width, hosts: make(map[string]int)}
		ns.groups[pattern] = g
	}
	g.list()
	return g
}

// merge adds the members of other. A host this set already holds keeps the
// spelling it has. With take, the groups of other are taken over rather
// than copied, and other must not be used again.
func (ns *NodeSet) merge(other *NodeSet, take bool) {
	for p, og := range other.groups {
		g, ok := ns.groups[p]
		switch {
		case !ok:
			if !take {
				og = og.clone()
			}
			ns.groups[p] = og
			continue
		case g.hosts == nil && og.hosts == nil:
			if u, ok := unionProducts(g, og); ok {
				ns.groups[p] = u
				continue
			}
		}
		g.list()
		og.each(g.add)
	}
}

func (ns *NodeSet) subtract(other *NodeSet) {
	for p, og := range other.groups {
		g, ok := ns.groups[p]
		switch {
		case !ok:
			continue
		case g.hosts == nil && og.hosts == nil:
			if d, ok := differenceProducts(g, og); ok {
				ns.put(p, d)
				continue
			}
		}
		g.list()
		if og.len() < len(g.hosts) {
			og.each(func(vals, _ []int) { g.remove(vals) })
		} else {
			g.keep(func(vals []int) bool { return !og.has(vals) })
		}
		ns.dropEmpty(p)
	}
}

func (ns *NodeSet) intersect(other *NodeSet) {
	for p, g := range ns.groups {
		og, ok := other.groups[p]
		switch {
		case !ok:
			delete(ns.groups, p)
		case g.hosts == nil && og.hosts == nil:
			ns.put(p, intersectProducts(g, og))
		default:
			g.list()
			g.keep(og.has)
			ns.dropEmpty(p)
		}
	}
}

// symmetricDifference toggles the members of other. With take, the groups
// of other are taken over rather than copied, as merge does.
func (ns *NodeSet) symmetricDifference(other *NodeSet, take bool) {
	for p, og := range other.groups {
		g, ok := ns.groups[p]
		switch {
		case !ok:
			if !take {
				og = og.clone()
			}
			ns.groups[p] = og
			continue
		case g.hosts == nil && og.hosts == nil:
			if x, ok := symmetricDifferenceProducts(g, og); ok {
				ns.put(p, x)
				continue
			}
		}
		g.list()
		og.each(func(vals, pads []int) {
			if g.has(vals) {
				g.remove(vals)
			} else {
				g.add(vals, pads)
			}
		})
		ns.dropEmpty(p)
	}
}

// put sets the group of a pattern, or removes it for nil.
func (ns *NodeSet) put(pattern string, g *group) {
	if g == nil {
		delete(ns.groups, pattern)
		return
	}
	ns.groups[pattern] = g
}

// dropEmpty removes the group of a pattern if it holds no host, so that a
// set holds a group only for a pattern it has hosts of.
func (ns *NodeSet) dropEmpty(pattern string) {
	if ns.groups[pattern].len() == 0 {
		delete(ns.groups, pattern)
	}
}

// Hostlist renders the set in the syntax Slurm and FreeIPMI accept: one
// bracketed range per name at most, and no steps.
//
// Each pattern is folded along the one dimension that gives the fewest names,
// so rack[1-2]node[001-100] becomes rack1node[001-100],rack2node[001-100]
// rather than two hundred names, and the argument stays short. Both parsers
// read several bracketed dimensions in one name as well, but this form is the
// one every version of them reads.
func (ns *NodeSet) Hostlist() string {
	var buf []byte
	for _, p := range sortedPatterns(ns.groups) {
		for _, v := range ns.groups[p].hostlist() {
			if len(buf) > 0 {
				buf = append(buf, ',')
			}
			buf = v.appendTo(buf, 0)
		}
	}
	return string(buf)
}
