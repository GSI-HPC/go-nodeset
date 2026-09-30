// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"slices"
	"sort"
	"strings"
)

// vector is one folded host name: a pattern and the set of values each of its
// dimensions takes.
type vector struct {
	pattern string
	dims    []*rangeSet
}

// folded is how the hosts of a group fold: the vectors String renders, and
// for a listed pattern with several numbers the space and boxes they come
// from, which Expand lists the hosts from.
type folded struct {
	vectors []vector
	sp      *ndSpace
	boxes   []box
}

// fold folds the hosts of the group, once until they change. A product is
// its one vector. A pattern with one number folds into one vector, its values
// in numeric order; one with several numbers folds as foldND says.
func (g *group) fold() *folded {
	if f := g.folded.Load(); f != nil {
		return f
	}
	f := &folded{}
	switch {
	case !g.listed():
		f.vectors = []vector{{pattern: g.pattern, dims: g.dims}}
	case g.width == 1:
		rs := &rangeSet{values: make([]int, 0, len(g.hosts)), pads: make([]int, 0, len(g.hosts))}
		for _, at := range g.hosts {
			rs.values = append(rs.values, g.data[at])
			rs.pads = append(rs.pads, g.data[at+1])
		}
		rs.sortUnique()
		f.vectors = []vector{{pattern: g.pattern, dims: []*rangeSet{rs}}}
	default:
		f.sp, f.boxes = foldND(g.nodes())
		f.vectors = f.sp.vectors(g.pattern, f.boxes)
	}
	g.folded.Store(f)
	return f
}

// fold renders the set, merging hosts into bracketed ranges.
func (ns *NodeSet) fold() string {
	var buf []byte
	for i, p := range sortedPatterns(ns.groups) {
		for j, v := range ns.groups[p].fold().vectors {
			if i > 0 || j > 0 {
				buf = append(buf, ',')
			}
			buf = v.appendTo(buf, ns.autostep)
		}
	}
	return string(buf)
}

// inOrder calls f for every host of the group in the order ClusterShell
// lists them: a pattern with one number in numeric order, and one with
// several vector by vector as the group folds, the last number varying
// fastest, each in the order of its text, shorter first. The slices are
// valid only during the call.
func (g *group) inOrder(f func(vals, pads []int)) {
	switch {
	case !g.listed() && g.width <= 1:
		g.walk(nil, f)
	case !g.listed():
		orders := make([][]int, g.width)
		for d, rs := range g.dims {
			texts := make([]coord, len(rs.values))
			for i, v := range rs.values {
				texts[i] = coord{text: format(v, rs.pads[i])}
			}
			orders[d] = make([]int, len(texts))
			for i := range orders[d] {
				orders[d][i] = i
			}
			slices.SortFunc(orders[d], func(a, b int) int { return compareCoord(texts[a], texts[b]) })
		}
		g.walk(orders, f)
	case g.width == 1:
		rs := g.fold().vectors[0].dims[0]
		for i := range rs.values {
			f(rs.values[i:i+1], rs.pads[i:i+1])
		}
	default:
		fd := g.fold()
		fd.sp.eachHost(fd.boxes, f)
	}
}

// appendName writes out the name of one host of the pattern.
func appendName(buf []byte, pattern string, vals, pads []int) []byte {
	return appendFilled(buf, pattern, func(buf []byte, d int) []byte {
		return appendFormat(buf, vals[d], pads[d])
	})
}

// appendFilled writes out pattern, with what fill writes for each numeric
// dimension in place of its %s, and % for %%.
func appendFilled(buf []byte, pattern string, fill func(buf []byte, d int) []byte) []byte {
	d := 0
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c != '%' {
			buf = append(buf, c)
			continue
		}
		i++
		if pattern[i] == '%' {
			buf = append(buf, '%')
			continue
		}
		buf = fill(buf, d)
		d++
	}
	return buf
}

// unitVectors makes one vector per host, each dimension holding the host's
// value with the width it was written with.
func unitVectors(pattern string, nodes []node) []vector {
	vectors := make([]vector, 0, len(nodes))
	for _, n := range nodes {
		dims := make([]*rangeSet, len(n.vals))
		for i, v := range n.vals {
			dims[i] = &rangeSet{values: []int{v}, pads: []int{n.pads[i]}}
		}
		vectors = append(vectors, vector{pattern: pattern, dims: dims})
	}
	return vectors
}

// hostlist folds the group as Hostlist writes it, as foldOneAxis folds its
// hosts. A product is not listed for it: foldOneAxis would unite the values
// of the dimension with the most values, the later of two with as many, and
// give a vector for each value of the others, in numeric order.
func (g *group) hostlist() []vector {
	if g.listed() {
		return foldOneAxis(g.pattern, g.nodes())
	}
	axis := 0
	for d, rs := range g.dims {
		if len(rs.values) >= len(g.dims[axis].values) {
			axis = d
		}
	}
	var out []vector
	dims := make([]*rangeSet, g.width)
	var rec func(d int)
	rec = func(d int) {
		switch d {
		case g.width:
			out = append(out, vector{pattern: g.pattern, dims: slices.Clone(dims)})
		case axis:
			dims[d] = g.dims[d]
			rec(d + 1)
		default:
			rs := g.dims[d]
			for i := range rs.values {
				dims[d] = &rangeSet{values: rs.values[i : i+1 : i+1], pads: rs.pads[i : i+1 : i+1]}
				rec(d + 1)
			}
		}
	}
	rec(0)
	return out
}

// foldOneAxis merges the listed hosts of one pattern along a single
// dimension, the one that leaves the fewest vectors, so every vector has at
// most one dimension with more than one value. A tie goes to the later
// dimension, which is the one that usually counts nodes. Steps are never
// used.
func foldOneAxis(pattern string, nodes []node) []vector {
	var best []vector
	for axis := range nodes[0].vals {
		merged := foldAxis(unitVectors(pattern, nodes), axis)
		if best == nil || len(merged) <= len(best) {
			best = merged
		}
	}
	sortVectors(best)
	return best
}

// foldAxis unions the values of one dimension across every pair of vectors
// that agree on all other dimensions.
//
// Values are collected first and put in order once per merged vector, so a
// pass costs O(n log n) however many vectors fold into one.
func foldAxis(vectors []vector, axis int) []vector {
	groups := make(map[string]int, len(vectors))
	out := make([]vector, 0, len(vectors))
	grown := make([]bool, 0, len(vectors))

	for _, v := range vectors {
		key := v.keyWithout(axis)
		if idx, ok := groups[key]; ok {
			target := out[idx].dims[axis]
			target.values = append(target.values, v.dims[axis].values...)
			target.pads = append(target.pads, v.dims[axis].pads...)
			grown[idx] = true
			continue
		}
		groups[key] = len(out)
		out = append(out, v)
		grown = append(grown, false)
	}
	for idx, g := range grown {
		if g {
			out[idx].dims[axis].sortUnique()
		}
	}
	return out
}

// keyWithout identifies a vector by every dimension except axis.
func (v vector) keyWithout(axis int) string {
	var b strings.Builder
	for i, d := range v.dims {
		if i == axis {
			b.WriteString("\x00*\x00")
			continue
		}
		b.WriteString(d.String(0))
		b.WriteByte(0)
	}
	return b.String()
}

// appendTo writes out the vector, the pattern filled with its rendered
// dimensions.
func (v vector) appendTo(buf []byte, autostep int) []byte {
	return appendFilled(buf, v.pattern, func(buf []byte, d int) []byte {
		return append(buf, v.dims[d].String(autostep)...)
	})
}

// sortVectors orders folded names by their lowest value in each dimension, so
// that output is deterministic.
//
// The vectors are those of one pattern, so they have the same number of
// dimensions, and no two hold the same host, so no two have the same lowest
// value in every dimension. Two vectors of one pattern without dimensions
// would be one host, so there are dimensions to compare whenever sorting
// compares anything.
func sortVectors(vectors []vector) {
	sort.Slice(vectors, func(i, j int) bool {
		a, b := vectors[i].dims, vectors[j].dims
		d := 0
		for d < len(a)-1 && a[d].values[0] == b[d].values[0] {
			d++
		}
		return a[d].values[0] < b[d].values[0]
	})
}
