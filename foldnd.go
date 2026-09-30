// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"encoding/binary"
	"slices"
	"sort"
	"strings"
)

// A set of names with several numbers has more than one folded form. The
// package writes the one ClusterShell writes, found the way ClusterShell's
// RangeSetND finds it: a set that is the product of its dimensions is one
// vector; any other set starts as one box per host, and boxes are merged in
// passes until a pass merges nothing. Two boxes merge when they differ in one
// dimension at most, and there either share no value or one holds the other;
// here boxes never share a host, since they start as one per host and merging
// unions them, so two boxes merge exactly when they differ in one dimension,
// and share no value there. Every pass sorts the boxes first. The first passes are linear: each box
// takes in the boxes that follow it for as long as they merge. After the
// first linear pass that merges nothing, every pass is full: each box takes
// in every later box that merges with it, in order.
//
// ClusterShell compares every later box in a full pass. The package finds
// the same boxes, in the same order, through an index of the boxes by all
// their dimensions but one, which keeps the cost of a pass near linear.

// coord is one value of a dimension, as the hosts spell it. Two spellings
// of one number are two values here, as they are in ClusterShell, which
// keeps a value as the text it was written as.
type coord struct {
	val, pad int
	text     string
}

// compareCoord orders values as ClusterShell's RangeSet does: shorter text
// first, then by the text, which for one width is numeric order.
func compareCoord(a, b coord) int {
	if c := cmp.Compare(len(a.text), len(b.text)); c != 0 {
		return c
	}
	return strings.Compare(a.text, b.text)
}

// ndSpace numbers the values of each dimension of a pattern: rank is a
// value's place in compareCoord order, and a dimension of a box is the
// ascending list of its ranks.
type ndSpace struct {
	coords [][]coord // by dimension, by rank
	text   [][]int   // by dimension, by rank: the place of the text in string order
}

// box is a vector being merged.
type box struct {
	dims [][]int
	size int
}

func newBox(dims [][]int) box {
	n := 1
	for _, d := range dims {
		n *= len(d)
	}
	return box{dims: dims, size: n}
}

// newSpace ranks the values the hosts take, and returns the hosts as ranks:
// host i holds ranks[i*d : i*d+d] for d dimensions, all in one slice.
func newSpace(nodes []node) (*ndSpace, []int) {
	dimCount := len(nodes[0].vals)
	sp := &ndSpace{coords: make([][]coord, dimCount), text: make([][]int, dimCount)}
	ranks := make([]int, len(nodes)*dimCount)
	type spelling struct{ val, pad int }
	for d := range dimCount {
		index := map[spelling]int{}
		for _, n := range nodes {
			k := spelling{n.vals[d], n.pads[d]}
			if _, ok := index[k]; !ok {
				index[k] = 0
				sp.coords[d] = append(sp.coords[d], coord{val: k.val, pad: k.pad, text: format(k.val, k.pad)})
			}
		}
		slices.SortFunc(sp.coords[d], compareCoord)
		for r, c := range sp.coords[d] {
			index[spelling{c.val, c.pad}] = r
		}
		byText := make([]int, len(sp.coords[d]))
		for r := range byText {
			byText[r] = r
		}
		sort.Slice(byText, func(i, j int) bool {
			return sp.coords[d][byText[i]].text < sp.coords[d][byText[j]].text
		})
		sp.text[d] = make([]int, len(byText))
		for place, r := range byText {
			sp.text[d][r] = place
		}
		for i, n := range nodes {
			ranks[i*dimCount+d] = index[spelling{n.vals[d], n.pads[d]}]
		}
	}
	return sp, ranks
}

// compare orders boxes as ClusterShell sorts them before every pass: larger
// boxes first, then dimension by dimension the one with more values, the
// lower first value and the lower last value, compared as text, as
// ClusterShell compares them there: 106 before 32. Two boxes that shared the
// first value of every dimension would share a host, so no two compare
// equal, and the order needs no stable sort.
func (sp *ndSpace) compare(a, b box) int {
	c := cmp.Compare(b.size, a.size)
	for d := 0; c == 0 && d < len(a.dims); d++ {
		x, y, text := a.dims[d], b.dims[d], sp.text[d]
		if c = cmp.Compare(len(y), len(x)); c == 0 {
			if c = cmp.Compare(text[x[0]], text[y[0]]); c == 0 {
				c = cmp.Compare(text[x[len(x)-1]], text[y[len(y)-1]])
			}
		}
	}
	return c
}

// sortUnits sorts boxes of one host each as compare does, which for them is
// by the text of each value, dimension by dimension: a radix sort, one
// counting pass per dimension from the last, in linear time.
func (sp *ndSpace) sortUnits(boxes []box) {
	tmp := make([]box, len(boxes))
	for d := len(sp.text) - 1; d >= 0; d-- {
		text := sp.text[d]
		count := make([]int, len(text)+1)
		for _, b := range boxes {
			count[text[b.dims[d][0]]+1]++
		}
		for i := 1; i < len(count); i++ {
			count[i] += count[i-1]
		}
		for _, b := range boxes {
			place := text[b.dims[d][0]]
			tmp[count[place]] = b
			count[place]++
		}
		boxes, tmp = tmp, boxes
	}
	if len(sp.text)%2 == 1 {
		copy(tmp, boxes)
	}
}

// join merges two boxes that differ in dimension at only. They share no
// value there, so the dimension of the merged box is the two lists joined.
func join(a, b box, at int) box {
	x, y := a.dims[at], b.dims[at]
	joined := make([]int, 0, len(x)+len(y))
	i, j := 0, 0
	for i < len(x) && j < len(y) {
		if x[i] < y[j] {
			joined = append(joined, x[i])
			i++
		} else {
			joined = append(joined, y[j])
			j++
		}
	}
	joined = append(append(joined, x[i:]...), y[j:]...)
	dims := slices.Clone(a.dims)
	dims[at] = joined
	return newBox(dims)
}

// key writes out the dimensions of a box other than d, into buf: two boxes
// differ in dimension d only, and so merge, exactly when their keys for d
// are the same.
func key(buf []byte, b box, d int) []byte {
	buf = buf[:0]
	for e, x := range b.dims {
		if e == d {
			continue
		}
		buf = binary.AppendUvarint(buf, uint64(len(x)))
		for _, v := range x {
			buf = binary.AppendUvarint(buf, uint64(v))
		}
	}
	return buf
}

// byKey indexes the boxes of a pass by their key for one dimension: the
// boxes of each key are a range of positions, ascending, in one slice.
type byKey struct {
	ranges    map[string][2]int
	positions []int
}

func newByKey(boxes []box, d int) byKey {
	keys := make([]string, len(boxes))
	var buf []byte
	for i, b := range boxes {
		buf = key(buf, b, d)
		keys[i] = string(buf)
	}
	positions := make([]int, len(boxes))
	for i := range positions {
		positions[i] = i
	}
	slices.SortStableFunc(positions, func(i, j int) int { return strings.Compare(keys[i], keys[j]) })
	ranges := make(map[string][2]int)
	for lo := 0; lo < len(positions); {
		hi := lo + 1
		for hi < len(positions) && keys[positions[hi]] == keys[positions[lo]] {
			hi++
		}
		ranges[keys[positions[lo]]] = [2]int{lo, hi}
		lo = hi
	}
	return byKey{ranges: ranges, positions: positions}
}

// after returns the positions of the boxes with key k, beyond from.
func (x byKey) after(k []byte, from int) []int {
	r := x.ranges[string(k)]
	positions := x.positions[r[0]:r[1]]
	return positions[sort.SearchInts(positions, from+1):]
}

// mergePasses merges boxes of one host each as ClusterShell's RangeSetND
// does. The boxes are sorted before every pass, but for a full pass that
// follows a linear one that merged nothing, which leaves them sorted.
func (sp *ndSpace) mergePasses(boxes []box) []box {
	sp.sortUnits(boxes)
	full := false
	for {
		var changed bool
		if full {
			boxes, changed = fullPass(boxes)
		} else {
			boxes, changed = linearPass(boxes)
		}
		if !changed {
			if full {
				return boxes
			}
			full = true
			continue
		}
		slices.SortFunc(boxes, sp.compare)
	}
}

// linearPass merges each box with the boxes that follow it, for as long as
// they merge.
func linearPass(boxes []box) ([]box, bool) {
	out := make([]box, 0, len(boxes))
	changed := false
	for i := 0; i < len(boxes); {
		g, j := newGrower(boxes[i]), i+1
		for ; j < len(boxes) && g.take(boxes[j]); j++ {
			changed = true
		}
		out = append(out, g.box())
		i = j
	}
	return out, changed
}

// grower is a box that takes in the boxes of a linear pass one by one. The
// dimension it grows keeps its values in a set, sorted once at the end, so
// that a run of n boxes costs O(n log n) rather than O(n²).
type grower struct {
	dims [][]int
	at   int // the dimension held in set, or -1
	set  map[int]bool
}

func newGrower(b box) *grower {
	return &grower{dims: slices.Clone(b.dims), at: -1}
}

// take merges b into the grower if the two differ in one dimension only.
func (g *grower) take(b box) bool {
	at := -1
	for d, y := range b.dims {
		if g.holds(d, y) {
			continue
		}
		if at >= 0 {
			return false
		}
		at = d
	}
	if at != g.at {
		g.flush()
		g.at, g.set = at, make(map[int]bool, len(g.dims[at]))
		for _, v := range g.dims[at] {
			g.set[v] = true
		}
	}
	for _, v := range b.dims[at] {
		g.set[v] = true
	}
	return true
}

// holds reports whether dimension d of the grower holds exactly the values y.
func (g *grower) holds(d int, y []int) bool {
	if d != g.at {
		return slices.Equal(g.dims[d], y)
	}
	if len(y) != len(g.set) {
		return false
	}
	for _, v := range y {
		if !g.set[v] {
			return false
		}
	}
	return true
}

// flush writes the set back into its dimension, sorted.
func (g *grower) flush() {
	if g.at < 0 {
		return
	}
	values := make([]int, 0, len(g.set))
	for v := range g.set {
		values = append(values, v)
	}
	sort.Ints(values)
	g.dims[g.at], g.at, g.set = values, -1, nil
}

func (g *grower) box() box {
	g.flush()
	return newBox(g.dims)
}

// fullPass merges each box with every later box that merges with it, in
// order: after a merge the box goes on from the box it took in. The later
// boxes that merge with a box are those that share one of its keys, so the
// pass looks them up in an index of the boxes by key, instead of trying
// every one.
func fullPass(boxes []box) ([]box, bool) {
	index := make([]byKey, len(boxes[0].dims))
	for d := range index {
		index[d] = newByKey(boxes, d)
	}

	gone := make([]bool, len(boxes))
	changed := false
	var buf []byte
	for i := range boxes {
		if gone[i] {
			continue
		}
		cur, from := boxes[i], i
		for {
			// The first later box still there that shares a key with the
			// box, over all its dimensions.
			next, at := -1, 0
			for d, x := range index {
				buf = key(buf, cur, d)
				for _, j := range x.after(buf, from) {
					if !gone[j] {
						if next < 0 || j < next {
							next, at = j, d
						}
						break
					}
				}
			}
			if next < 0 {
				break
			}
			cur, from, gone[next], changed = join(cur, boxes[next], at), next, true, true
		}
		boxes[i] = cur
	}

	out := boxes[:0]
	for i, b := range boxes {
		if !gone[i] {
			out = append(out, b)
		}
	}
	return out, changed
}

// foldND folds the hosts of a pattern with several dimensions into boxes, in
// the order ClusterShell writes them. It returns the hosts as ranks too, as
// newSpace gives them.
func foldND(nodes []node) (*ndSpace, []box, []int) {
	sp, ranks := newSpace(nodes)
	product := make([][]int, len(sp.coords))
	n := 1
	for d, values := range sp.coords {
		n *= len(values)
		if n > len(nodes) {
			break
		}
		product[d] = make([]int, len(values))
		for r := range product[d] {
			product[d][r] = r
		}
	}
	if n == len(nodes) {
		return sp, []box{newBox(product)}, ranks
	}
	// One box per host, its dimensions sliced from the host's ranks, and
	// every box's list of dimensions sliced from one slice.
	dimCount := len(sp.coords)
	boxes := make([]box, len(nodes))
	dims := make([][]int, len(ranks))
	for i := range boxes {
		for d := range dimCount {
			k := i*dimCount + d
			dims[k] = ranks[k : k+1 : k+1]
		}
		boxes[i] = box{dims: dims[i*dimCount : (i+1)*dimCount : (i+1)*dimCount], size: 1}
	}
	return sp, sp.mergePasses(boxes), ranks
}

// foldVectors folds the hosts of a pattern with several dimensions into
// vectors to render, each dimension in numeric order, as one of a single
// number is, so that a range never mixes widths.
func foldVectors(pattern string, nodes []node) []vector {
	sp, boxes, _ := foldND(nodes)
	out := make([]vector, len(boxes))
	for i, b := range boxes {
		dims := make([]*rangeSet, len(b.dims))
		for d, ranks := range b.dims {
			values := make([]coord, len(ranks))
			for k, r := range ranks {
				values[k] = sp.coords[d][r]
			}
			slices.SortStableFunc(values, func(x, y coord) int { return cmp.Compare(x.val, y.val) })
			rs := &rangeSet{}
			for _, c := range values {
				rs.values = append(rs.values, c.val)
				rs.pads = append(rs.pads, c.pad)
			}
			dims[d] = rs
		}
		out[i] = vector{pattern: pattern, dims: dims}
	}
	return out
}

// orderND puts the hosts of a pattern with several dimensions, in place, in
// the order ClusterShell lists them: box by box as foldND gives the boxes,
// each box with its last dimension varying fastest. Each host is found by a
// binary search over the hosts sorted by their ranks, so the order costs
// two slices of indexes and no strings.
func orderND(nodes []node) {
	_, boxes, ranks := foldND(nodes)
	dimCount := len(nodes[0].vals)
	of := func(i int) []int { return ranks[i*dimCount : (i+1)*dimCount] }
	byRank := make([]int, len(nodes))
	for i := range byRank {
		byRank[i] = i
	}
	slices.SortFunc(byRank, func(a, b int) int { return slices.Compare(of(a), of(b)) })

	// from[k] is the host that goes to place k.
	from := make([]int, 0, len(nodes))
	tuple := make([]int, dimCount)
	idx := make([]int, dimCount)
	for _, b := range boxes {
		clear(idx)
		for {
			for d, i := range idx {
				tuple[d] = b.dims[d][i]
			}
			k, _ := slices.BinarySearchFunc(byRank, tuple, func(i int, t []int) int { return slices.Compare(of(i), t) })
			from = append(from, byRank[k])
			d := dimCount - 1
			for ; d >= 0; d-- {
				if idx[d]++; idx[d] < len(b.dims[d]) {
					break
				}
				idx[d] = 0
			}
			if d < 0 {
				break
			}
		}
	}

	// Move the hosts along the cycles of the permutation.
	for start := range from {
		if from[start] < 0 {
			continue
		}
		held, k := nodes[start], start
		for {
			next := from[k]
			from[k] = -1
			if next == start {
				nodes[k] = held
				break
			}
			nodes[k], k = nodes[next], next
		}
	}
}
