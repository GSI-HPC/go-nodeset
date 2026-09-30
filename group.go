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

// intersectProduct returns the hosts two products share, which is the
// product of the values their dimensions share, each spelled as in g.
func (g *group) intersectProduct(o *group) *group {
	dims := make([]*rangeSet, g.width)
	for d, rs := range g.dims {
		shared := &rangeSet{}
		for i, v := range rs.values {
			if _, ok := slices.BinarySearch(o.dims[d].values, v); ok {
				shared.values = append(shared.values, v)
				shared.pads = append(shared.pads, rs.pads[i])
			}
		}
		dims[d] = shared
	}
	return newProduct(g.pattern, dims)
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
