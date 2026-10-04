// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// rangeSet is an ordered set of numbers forming one dimension of a host name.
//
// Padding is not part of a host's identity, so exe1 and exe01 are one host,
// but every value keeps the width it was written with: pads[i] is the width
// of values[i], and a set never shows a host under a name it was not given.
type rangeSet struct {
	values []int
	pads   []int
}

// padOf reports the display width a numeric literal asks for. Only a leading
// zero counts as padding, so "7" and "10" ask for none and "007" asks for
// three.
func padOf(lit string) int {
	if len(lit) > 1 && lit[0] == '0' {
		return len(lit)
	}
	return 0
}

// digits reports how many digits n has without padding.
func digits(n int) int {
	d := 1
	for ; n >= 10; n /= 10 {
		d++
	}
	return d
}

// normalPad drops a width that does not change how n is shown, so that each
// spelling of a number has exactly one width: 10 at width two is "10", the
// same as 10 at width zero.
func normalPad(n, pad int) int {
	if pad <= digits(n) {
		return 0
	}
	return pad
}

// format renders one number with its padding.
func format(n, pad int) string {
	if pad == 0 {
		return strconv.Itoa(n)
	}
	return string(appendFormat(nil, n, pad))
}

// appendFormat writes out one number with its padding.
func appendFormat(buf []byte, n, pad int) []byte {
	for i := digits(n); i < pad; i++ {
		buf = append(buf, '0')
	}
	return strconv.AppendInt(buf, int64(n), 10)
}

// fits reports whether a value shown with width pad reads the same when shown
// with width run, so that it can share a range with values of that width.
func fits(n, pad, run int) bool {
	return pad == run || (pad == 0 && digits(n) >= run)
}

// add appends one value with its width.
func (rs *rangeSet) add(n, pad int) {
	rs.values = append(rs.values, n)
	rs.pads = append(rs.pads, normalPad(n, pad))
}

// sortUnique orders the values and drops repeats. Of two spellings of one
// number, the one given first is kept.
func (rs *rangeSet) sortUnique() {
	ascending := true
	for i := 1; i < len(rs.values) && ascending; i++ {
		ascending = rs.values[i-1] < rs.values[i]
	}
	if ascending {
		return
	}
	type spelled struct{ val, pad, at int }
	all := make([]spelled, len(rs.values))
	for i, v := range rs.values {
		all[i] = spelled{v, rs.pads[i], i}
	}
	slices.SortFunc(all, func(a, b spelled) int { return cmp.Or(cmp.Compare(a.val, b.val), cmp.Compare(a.at, b.at)) })
	values, pads := rs.values[:0], rs.pads[:0]
	for _, s := range all {
		if n := len(values); n > 0 && values[n-1] == s.val {
			continue
		}
		values = append(values, s.val)
		pads = append(pads, s.pad)
	}
	rs.values, rs.pads = values, pads
}

// combine walks two sets in order and keeps the values that only rs holds,
// that only o holds, or that both hold, as the three flags say. A value keeps
// the width it has in the set it comes from, and in rs when both hold it.
func (rs *rangeSet) combine(o *rangeSet, onlyRS, onlyO, both bool) *rangeSet {
	size := 0
	if onlyRS || both {
		size += len(rs.values)
	}
	if onlyO {
		size += len(o.values)
	}
	out := &rangeSet{values: make([]int, 0, size), pads: make([]int, 0, size)}
	keep := func(from *rangeSet, i int) {
		out.values = append(out.values, from.values[i])
		out.pads = append(out.pads, from.pads[i])
	}
	i, j := 0, 0
	for i < len(rs.values) || j < len(o.values) {
		switch {
		case j == len(o.values) || i < len(rs.values) && rs.values[i] < o.values[j]:
			if onlyRS {
				keep(rs, i)
			}
			i++
		case i == len(rs.values) || o.values[j] < rs.values[i]:
			if onlyO {
				keep(o, j)
			}
			j++
		default:
			if both {
				keep(rs, i)
			}
			i, j = i+1, j+1
		}
	}
	return out
}

// overlap reports whether rs holds a value o does not, and whether the two
// share a value.
func (rs *rangeSet) overlap(o *rangeSet) (beyond, shared bool) {
	j := 0
	for _, v := range rs.values {
		for j < len(o.values) && o.values[j] < v {
			j++
		}
		if j < len(o.values) && o.values[j] == v {
			shared = true
		} else {
			beyond = true
		}
	}
	return beyond, shared
}

// equal reports whether two sets hold the same values, spelled alike.
func (rs *rangeSet) equal(o *rangeSet) bool {
	return slices.Equal(rs.values, o.values) && slices.Equal(rs.pads, o.pads)
}

// span is one part of a bracket expression, "1-10/2", before it is expanded.
type span struct {
	start, end, step, pad int
}

// count reports how many values the span expands to.
func (s span) count() int { return (s.end-s.start)/s.step + 1 }

// parseSpans reads the contents of a bracket expression, "1-10/2,20", without
// expanding it, and reports how many values it names in total. Every value
// counts, including repeats, so a range written as many parts weighs what one
// written as a single part does.
func parseSpans(spec string) ([]span, int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, 0, fmt.Errorf("empty range")
	}
	var (
		spans []span
		total int
	)
	for part := range strings.SplitSeq(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, 0, fmt.Errorf("empty range element in %q", spec)
		}
		step := 1
		if slash := strings.IndexByte(part, '/'); slash >= 0 {
			var err error
			step, err = parseNumber(part[slash+1:])
			if err != nil || step < 1 {
				return nil, 0, fmt.Errorf("invalid step %q in range %q", part[slash+1:], spec)
			}
			part = part[:slash]
		}
		lo, hi, isRange := strings.Cut(part, "-")
		start, err := parseNumber(lo)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid range bound %q in %q", lo, spec)
		}
		end := start
		if isRange {
			// exe[1-$N] with N empty arrives as 1-, which is no range of
			// one value: it would select exe1 where several were meant.
			if hi == "" {
				return nil, 0, fmt.Errorf("the range %q has no last bound", part)
			}
			if end, err = parseNumber(hi); err != nil {
				return nil, 0, fmt.Errorf("invalid range bound %q in %q", hi, spec)
			}
			if end < start {
				return nil, 0, fmt.Errorf("descending range %q", part)
			}
			// The padding of the first bound applies to the whole range, so
			// a last bound written with another width would be shown under
			// a name it was not given: 1-010 would print 010 as 10.
			if (padOf(lo) > 0 && len(hi) < len(lo)) || (padOf(hi) > 0 && len(hi) != len(lo)) {
				return nil, 0, fmt.Errorf("the bounds of %q are padded to different widths", part)
			}
		} else if step != 1 {
			return nil, 0, fmt.Errorf("step given for the single value %q", part)
		}
		s := span{start: start, end: end, step: step, pad: padOf(lo)}
		spans = append(spans, s)
		// Both terms are at most maxRangeElements+1 here, so the sum cannot
		// overflow however many parts are written.
		if total += s.count(); total > maxRangeElements {
			return nil, 0, fmt.Errorf("range %q expands to more than %d elements", spec, maxRangeElements)
		}
	}
	return spans, total, nil
}

// expandSpans turns parsed spans into the dimension they name.
func expandSpans(spans []span, total int) *rangeSet {
	rs := &rangeSet{values: make([]int, 0, total), pads: make([]int, 0, total)}
	for _, s := range spans {
		for n := s.start; n <= s.end; n += s.step {
			rs.add(n, s.pad)
		}
	}
	rs.sortUnique()
	return rs
}

// parseNumber reads a decimal literal, rejecting anything else.
func parseNumber(lit string) (int, error) {
	if lit == "" || len(lit) > 18 {
		return 0, fmt.Errorf("invalid number %q", lit)
	}
	n := 0
	for i := 0; i < len(lit); i++ {
		if lit[i] < '0' || lit[i] > '9' {
			return 0, fmt.Errorf("invalid number %q", lit)
		}
		n = n*10 + int(lit[i]-'0')
	}
	return n, nil
}

// String renders the dimension. A single value is rendered bare so that it
// reads as part of the host name; anything else is bracketed.
func (rs *rangeSet) String(autostep int) string {
	if len(rs.values) == 1 {
		return format(rs.values[0], rs.pads[0])
	}
	return "[" + rs.list(autostep) + "]"
}

// list renders the values as comma separated ranges, without the brackets.
// A range only joins values that read the same at the width of its first
// value, so that parsing the output gives every value back its own width.
//
// It walks the values once, left to right, as ClusterShell's RangeSet does,
// so that the two fold alike: a run grows while its step stays the same, and
// when the step changes the run is written as a range if its step is 1 or it
// holds at least autostep values, and value by value otherwise, its last value
// then starting the next run. Below 2, autostep writes only runs of step 1 as
// ranges.
func (rs *rangeSet) list(autostep int) string {
	vals, pads := rs.values, rs.pads
	var parts []string
	single := func(i int) { parts = append(parts, format(vals[i], pads[i])) }
	stepped := func(first, last, step int) bool {
		return step == 1 || (autostep >= 2 && (vals[last]-vals[first])/step+1 >= autostep)
	}
	run := func(first, last, step int) {
		if first == last {
			single(first)
			return
		}
		pad := pads[first]
		r := format(vals[first], pad) + "-" + format(vals[last], pad)
		if step > 1 {
			r += "/" + strconv.Itoa(step)
		}
		parts = append(parts, r)
	}

	// The pending run holds the values from first to last; step is 0 while
	// it holds one value.
	first, last, step := 0, 0, 0
	for k := 1; k < len(vals); k++ {
		gap := vals[k] - vals[last]
		width := !fits(vals[k], pads[k], pads[first])
		if !width && (step == 0 || gap == step) {
			last, step = k, gap
			continue
		}
		switch {
		case step == 0:
			single(first)
			first, step = k, 0
		case stepped(first, last, step):
			run(first, last, step)
			first, step = k, 0
			if !width {
				// ClusterShell carries the gap to the new value on as the
				// step of the run it starts.
				step = gap
			}
		case width:
			for i := first; i <= last; i++ {
				single(i)
			}
			first, step = k, 0
		default:
			for i := first; i < last; i++ {
				single(i)
			}
			first, step = last, gap
		}
		last = k
	}
	switch {
	case step == 0:
		single(first)
	case stepped(first, last, step):
		run(first, last, step)
	default:
		for i := first; i <= last; i++ {
			single(i)
		}
	}
	return strings.Join(parts, ",")
}
