// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"encoding/binary"
	"maps"
	"slices"
	"sync/atomic"
)

// group is the hosts of a set that share one pattern: the name with a %s for
// each numeric dimension. It holds them in one of two forms. A product of
// ranges, as a bracketed name gives it, is kept as the ranges, dims, and its
// hosts are never listed one by one; any other set of hosts is listed in
// hosts, which maps a key of each host's values to where its values and
// widths start in data. Widths are not part of a host's identity, so the key
// holds the values only.
//
// The ranges of a product are shared between copies and never changed: an
// operation that changes the hosts of a product lists them first.
type group struct {
	pattern string
	width   int

	dims  []*rangeSet
	hosts map[string]int
	data  []int

	// folded caches how the hosts fold, for String and Expand. Readers
	// store it without a lock, so a set may be read from several goroutines
	// at once: two that fold at the same time store the same fold.
	folded atomic.Pointer[folded]
}

// node is one host of a group: the value of each dimension and the width it
// was written with. Its slices are views into the group's storage.
type node struct {
	vals, pads []int
}

// appendKey writes out a host's values, as the key it has in its group.
func appendKey(buf []byte, vals []int) []byte {
	for _, v := range vals {
		buf = binary.AppendUvarint(buf, uint64(v))
	}
	return buf
}

func newProduct(pattern string, dims []*rangeSet) *group {
	return &group{pattern: pattern, width: len(dims), dims: dims}
}

// listed reports whether the group lists its hosts. A group without
// dimensions holds a single host, whatever its form.
func (g *group) listed() bool { return g.hosts != nil && g.width > 0 }

// len reports how many hosts the group holds.
func (g *group) len() int {
	if g.hosts != nil {
		return len(g.hosts)
	}
	n := 1
	for _, d := range g.dims {
		n *= len(d.values)
	}
	return n
}

// clone returns a copy that can be changed on its own. A product shares its
// ranges, which are never changed, and a listed group is packed, dropping
// what removed hosts left in data. The fold is the same, so the copy keeps
// it.
func (g *group) clone() *group {
	out := &group{pattern: g.pattern, width: g.width, dims: g.dims}
	if g.hosts != nil {
		out.hosts = make(map[string]int, len(g.hosts))
		out.data = make([]int, 0, len(g.hosts)*2*g.width)
		for k, at := range g.hosts {
			out.hosts[k] = len(out.data)
			out.data = append(out.data, g.data[at:at+2*g.width]...)
		}
	}
	out.folded.Store(g.folded.Load())
	return out
}

// list turns a product into the list of its hosts, before the hosts change.
func (g *group) list() {
	if g.hosts != nil {
		return
	}
	hosts := make(map[string]int, g.len())
	data := make([]int, 0, g.len()*2*g.width)
	var buf []byte
	g.walk(nil, func(vals, pads []int) {
		buf = appendKey(buf[:0], vals)
		hosts[string(buf)] = len(data)
		data = append(append(data, vals...), pads...)
	})
	g.hosts, g.data, g.dims = hosts, data, nil
}

// each calls f for every host, in no particular order. The slices are
// valid only during the call.
func (g *group) each(f func(vals, pads []int)) {
	if g.hosts != nil {
		for _, at := range g.hosts {
			f(g.data[at:at+g.width], g.data[at+g.width:at+2*g.width])
		}
		return
	}
	g.walk(nil, f)
}

// walk calls f for every host of a product, the last dimension varying
// fastest. orders[d], if orders is not nil, lists the indexes of the values
// of dimension d in the order to take them; otherwise they are taken in
// numeric order.
func (g *group) walk(orders [][]int, f func(vals, pads []int)) {
	vals := make([]int, g.width)
	pads := make([]int, g.width)
	var rec func(d int)
	rec = func(d int) {
		if d == g.width {
			f(vals, pads)
			return
		}
		rs := g.dims[d]
		for k := range rs.values {
			i := k
			if orders != nil {
				i = orders[d][k]
			}
			vals[d], pads[d] = rs.values[i], rs.pads[i]
			rec(d + 1)
		}
	}
	rec(0)
}

// find returns the widths the group holds a host with, if it holds it.
func (g *group) find(vals []int) ([]int, bool) {
	if g.hosts != nil {
		at, ok := g.hosts[string(appendKey(make([]byte, 0, 32), vals))]
		if !ok {
			return nil, false
		}
		return g.data[at+g.width : at+2*g.width], true
	}
	pads := make([]int, g.width)
	for d, rs := range g.dims {
		i, ok := slices.BinarySearch(rs.values, vals[d])
		if !ok {
			return nil, false
		}
		pads[d] = rs.pads[i]
	}
	return pads, true
}

// has reports whether the group holds a host.
func (g *group) has(vals []int) bool {
	if g.hosts != nil {
		_, ok := g.hosts[string(appendKey(make([]byte, 0, 32), vals))]
		return ok
	}
	for d, rs := range g.dims {
		if _, ok := slices.BinarySearch(rs.values, vals[d]); !ok {
			return false
		}
	}
	return true
}

// add adds a host the group does not hold yet; one it holds keeps its
// spelling. The group must be listed.
func (g *group) add(vals, pads []int) {
	k := appendKey(make([]byte, 0, 32), vals)
	if _, ok := g.hosts[string(k)]; ok {
		return
	}
	g.hosts[string(k)] = len(g.data)
	g.data = append(append(g.data, vals...), pads...)
	g.folded.Store(nil)
}

// remove drops a host. The group must be listed.
func (g *group) remove(vals []int) {
	k := appendKey(make([]byte, 0, 32), vals)
	if _, ok := g.hosts[string(k)]; ok {
		delete(g.hosts, string(k))
		g.folded.Store(nil)
	}
}

// keep drops the hosts for which pred is false. The group must be listed.
func (g *group) keep(pred func(vals []int) bool) {
	for k, at := range g.hosts {
		if !pred(g.data[at : at+g.width]) {
			delete(g.hosts, k)
		}
	}
	g.folded.Store(nil)
}

// The hosts two products of one pattern name together are a product again
// in the cases below, and are computed range by range, without listing the
// hosts. Each returns false in any other case, and a nil group for no hosts.
// A host keeps the spelling of the product it comes from, and of a when both
// hold it, as the operators on listed hosts keep it.

// sameBut returns the one dimension in which two products differ, in values
// or in widths, or -1 if they differ in none. It returns false if they differ
// in more than one.
func sameBut(a, b *group) (int, bool) {
	at := -1
	for d, rs := range a.dims {
		if rs.equal(b.dims[d]) {
			continue
		}
		if at >= 0 {
			return 0, false
		}
		at = d
	}
	return at, true
}

// withDim returns a copy of the product a with dimension d replaced, or nil
// if the new dimension holds no value.
func withDim(a *group, d int, rs *rangeSet) *group {
	if len(rs.values) == 0 {
		return nil
	}
	dims := slices.Clone(a.dims)
	dims[d] = rs
	return newProduct(a.pattern, dims)
}

// unionProducts: when b holds no host a does not, the union is a; when the
// two differ in one dimension, spellings included, it is a with that
// dimension united.
func unionProducts(a, b *group) (*group, bool) {
	within := true
	for d, rs := range b.dims {
		if beyond, _ := rs.overlap(a.dims[d]); beyond {
			within = false
		}
	}
	if within {
		return a, true
	}
	at, ok := sameBut(a, b)
	if !ok {
		return nil, false
	}
	return withDim(a, at, a.dims[at].combine(b.dims[at], true, true, true)), true
}

// differenceProducts: when the two share no value in some dimension, the
// difference is a; when a holds values b does not in one dimension only, it
// is a with b's values taken out of that dimension.
func differenceProducts(a, b *group) (*group, bool) {
	at := -1
	for d, rs := range a.dims {
		beyond, shared := rs.overlap(b.dims[d])
		if !shared {
			return a, true
		}
		if !beyond {
			continue
		}
		if at >= 0 {
			return nil, false
		}
		at = d
	}
	if at < 0 {
		return nil, true
	}
	return withDim(a, at, a.dims[at].combine(b.dims[at], true, false, false)), true
}

// intersectProducts: the hosts two products share are always the product of
// the values their dimensions share.
func intersectProducts(a, b *group) *group {
	out := a
	for d, rs := range a.dims {
		if out = withDim(out, d, rs.combine(b.dims[d], false, false, true)); out == nil {
			return nil
		}
	}
	return out
}

// symmetricDifferenceProducts: when the two differ in one dimension,
// spellings included, the hosts in one of them only are a with the values of
// that dimension in one of them only; when they differ in none, there are
// none.
func symmetricDifferenceProducts(a, b *group) (*group, bool) {
	at, ok := sameBut(a, b)
	switch {
	case !ok:
		return nil, false
	case at < 0:
		return nil, true
	}
	return withDim(a, at, a.dims[at].combine(b.dims[at], true, true, false)), true
}

// nodes returns the hosts as nodes, in no particular order.
func (g *group) nodes() []node {
	out := make([]node, 0, g.len())
	if g.hosts != nil {
		for _, at := range g.hosts {
			out = append(out, node{vals: g.data[at : at+g.width : at+g.width], pads: g.data[at+g.width : at+2*g.width : at+2*g.width]})
		}
		return out
	}
	flat := make([]int, 0, g.len()*2*g.width)
	g.each(func(vals, pads []int) {
		flat = append(append(flat, vals...), pads...)
		end := len(flat)
		out = append(out, node{vals: flat[end-2*g.width : end-g.width : end-g.width], pads: flat[end-g.width : end : end]})
	})
	return out
}

// sortedPatterns returns the patterns of a set's groups in order.
func sortedPatterns(groups map[string]*group) []string {
	return slices.Sorted(maps.Keys(groups))
}
