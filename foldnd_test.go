// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// refFold is ClusterShell's RangeSetND folding, written out as plainly as
// ClusterShell writes it: every dimension a sorted list of texts, every
// full pass comparing every pair. foldND must give the same boxes in the
// same order, for all its indexes and sets.
func refFold(points [][]string) [][][]string {
	less := func(a, b string) int {
		return compareCoord(coord{text: a}, coord{text: b})
	}
	size := func(v [][]string) int {
		n := 1
		for _, d := range v {
			n *= len(d)
		}
		return n
	}
	key := func(a, b [][]string) int {
		if c := cmp.Compare(size(b), size(a)); c != 0 {
			return c
		}
		for d := range a {
			x, y := a[d], b[d]
			if c := cmp.Compare(len(y), len(x)); c != 0 {
				return c
			}
			if c := strings.Compare(x[0], y[0]); c != 0 {
				return c
			}
			if c := strings.Compare(x[len(x)-1], y[len(y)-1]); c != 0 {
				return c
			}
		}
		return 0
	}
	common := func(x, y []string) int {
		n := 0
		for _, v := range x {
			if slices.Contains(y, v) {
				n++
			}
		}
		return n
	}
	try := func(a, b [][]string) ([][]string, bool) {
		out := make([][]string, len(a))
		diff := 0
		for d := range a {
			x, y := a[d], b[d]
			n := common(x, y)
			switch {
			case n == len(x) && n == len(y):
				out[d] = x
				continue
			case n == 0:
				out[d] = slices.SortedFunc(slices.Values(slices.Concat(x, y)), less)
			case n == len(y):
				out[d] = x
			case n == len(x):
				out[d] = y
			default:
				return nil, false
			}
			if diff++; diff > 1 {
				return nil, false
			}
		}
		return out, true
	}

	var vecs [][][]string
	for _, p := range points {
		v := make([][]string, len(p))
		for d, t := range p {
			v[d] = []string{t}
		}
		vecs = append(vecs, v)
	}
	full := false
	for chg := true; chg; {
		chg = false
		slices.SortStableFunc(vecs, key)
		for i1 := 0; i1+1 < len(vecs); i1++ {
			for i2 := i1 + 1; i2 < len(vecs); {
				merged, ok := try(vecs[i1], vecs[i2])
				if ok {
					vecs[i1], chg = merged, true
					vecs = slices.Delete(vecs, i2, i2+1)
					continue
				}
				if !full {
					break
				}
				i2++
			}
		}
		if !chg && !full {
			chg, full = true, true
		}
	}
	return vecs
}

func TestFoldNDMatchesReference(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(3, 4))
	for trial := range 3000 {
		dims := 2 + rng.IntN(3)
		side := 2 + rng.IntN(12)
		count := 1 + rng.IntN(side*side)
		seen := map[string]bool{}
		var nodes []node
		var points [][]string
		for range count {
			n := node{vals: make([]int, dims), pads: make([]int, dims)}
			for d := range n.vals {
				n.vals[d] = rng.IntN(side) * (1 + rng.IntN(3)) * 7
				if rng.IntN(8) == 0 {
					n.pads[d] = normalPad(n.vals[d], 3)
				}
			}
			k := fmt.Sprint(n.vals)
			if seen[k] {
				continue
			}
			seen[k] = true
			nodes = append(nodes, n)
			p := make([]string, dims)
			for d := range p {
				p[d] = format(n.vals[d], n.pads[d])
			}
			points = append(points, p)
		}

		sp, boxes := foldND(nodes)
		got := make([][][]string, len(boxes))
		for i, b := range boxes {
			got[i] = make([][]string, len(b.dims))
			for d, ranks := range b.dims {
				for _, r := range ranks {
					got[i][d] = append(got[i][d], sp.coords[d][r].text)
				}
			}
		}
		want := refFold(points)
		if len(want) == 1 || isProduct(points) {
			// ClusterShell keeps a product as the one vector it is.
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("trial %d: foldND gives %v, the reference %v", trial, got, want)
		}
	}
}

// isProduct reports whether points are the product of their dimensions,
// which foldND folds into one box without merging.
func isProduct(points [][]string) bool {
	n := 1
	for d := range points[0] {
		values := map[string]bool{}
		for _, p := range points {
			values[p[d]] = true
		}
		n *= len(values)
	}
	return n == len(points)
}

// TestLinearPassGrowsOneDimension follows a linear pass through a run whose
// boxes differ in one dimension and then another: the grower compares the
// dimension it grows as a set, and moves on to the next.
func TestLinearPassGrowsOneDimension(t *testing.T) {
	t.Parallel()

	unit := func(x, y []int) box { return newBox([][]int{x, y}) }
	tests := []struct {
		boxes []box
		want  string
	}{
		// y grows to {1,2}, then a box holding {1,2} in y grows x.
		{[]box{unit([]int{1}, []int{1}), unit([]int{1}, []int{2}), unit([]int{2}, []int{1, 2})}, "[{[[1 2] [1 2]] 4}]"},
		// A box with as many values in y, but others, differs in both.
		{[]box{unit([]int{1}, []int{1}), unit([]int{1}, []int{2}), unit([]int{2}, []int{1, 3})}, "[{[[1] [1 2]] 2} {[[2] [1 3]] 2}]"},
		// And one with fewer values in y differs in both as well.
		{[]box{unit([]int{1}, []int{1}), unit([]int{1}, []int{2}), unit([]int{2}, []int{3})}, "[{[[1] [1 2]] 2} {[[2] [3]] 1}]"},
	}
	for i, tc := range tests {
		got, changed := linearPass(tc.boxes)
		if s := fmt.Sprint(got); s != tc.want || !changed {
			t.Errorf("case %d: linearPass = %s, %v, want %s, true", i, s, changed, tc.want)
		}
	}
}
