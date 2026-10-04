// SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"fmt"
	"strings"
)

// The expansion limits, so that a typo such as exe[1-100000000] is reported
// before it exhausts memory. They are variables only so that tests can lower
// them.
var (
	// maxRangeElements caps how far a single bracket range may expand.
	maxRangeElements = 1 << 20
	// maxSetElements caps how many hosts a whole expression may name, at
	// every step of its evaluation, and also how many hosts all of its
	// terms may name together, groups included. The second cap bounds the
	// work an expression costs, which the first alone does not:
	// a[1-600000]!a[1-600000] repeated is small at every step.
	maxSetElements = 1 << 20
)

// budget is what is left of the hosts an expression may name across all its
// terms. It is shared by the groups the expression refers to.
type budget struct{ left int }

func newBudget() *budget { return &budget{left: maxSetElements} }

// exceeded reports that the hosts a term names do not fit in what is left
// of the budget.
func (b *budget) exceeded(term string) error {
	if b.left < maxSetElements {
		return fmt.Errorf("at %q: the expression's terms name more than %d hosts together", term, maxSetElements)
	}
	return fmt.Errorf("%q expands to more than %d hosts", term, maxSetElements)
}

type operator byte

const (
	opUnion        operator = ','
	opDifference   operator = '!'
	opIntersection operator = '&'
	opSymmetric    operator = '^'
)

// parseExpression evaluates a full node set expression left to right.
func parseExpression(expr string, res Resolver, depth int, b *budget) (*NodeSet, error) {
	if depth > maxGroupDepth {
		return nil, fmt.Errorf("group references nested more than %d levels deep", maxGroupDepth)
	}
	result := New()
	var plain plainName
	err := splitTerms(expr, func(op operator, term string) error {
		if op == opUnion && term[0] != '@' && strings.IndexByte(term, '[') < 0 {
			// A name without brackets is one host, added as it is,
			// without a set of its own.
			if err := plain.parse(term, b); err != nil {
				return err
			}
			g, ok := result.groups[string(plain.pattern)]
			if !ok {
				g = result.listed(string(plain.pattern), len(plain.vals))
			}
			g.list()
			g.add(plain.vals, plain.pads)
			return nil
		}
		ts, err := parseTerm(term, res, depth, b)
		if err != nil {
			return err
		}
		if result.IsEmpty() && op == opUnion {
			// The first term needs no copy; the set is taken as it is.
			result = ts
		} else {
			apply(result, op, ts)
		}
		// The result needs no cap of its own: it holds no more hosts
		// than its terms named together, which b already caps.
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// splitTerms reads an expression term by term, left to right, and hands
// each term, trimmed and not empty, to each with the operator that joins it
// to the terms before it.
//
// Whitespace and the operators outside brackets end a term. A comma or
// whitespace between two operands is a union, and an empty operand of a
// union is nothing: "a,,b" and "a," are accepted. The other operators need
// an operand on both sides. "a&" is what a command substitution that
// printed nothing leaves behind, and evaluating it as "a" would select every
// host of a instead of none, so it is an error, as it is in ClusterShell.
//
// It stops at the first error, its own or one each returns, so each is
// given the terms before a syntax error and none after it.
func splitTerms(expr string, each func(op operator, term string) error) error {
	op := opUnion
	// operand reports whether the last thing read was an operand, and
	// pendingOp whether a set operator is waiting for its right operand.
	operand, pendingOp := false, false
	// start is where the term being read begins.
	start := 0
	depthBracket := 0

	flush := func(end int) error {
		term := strings.TrimSpace(expr[start:end])
		start = end + 1
		if term == "" {
			return nil
		}
		if err := each(op, term); err != nil {
			return err
		}
		op, operand, pendingOp = opUnion, true, false
		return nil
	}

	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case c == '[':
			depthBracket++
		case c == ']':
			depthBracket--
			if depthBracket < 0 {
				return fmt.Errorf("unbalanced ] in %q", expr)
			}
		case depthBracket > 0:
			// Whatever is in brackets belongs to the term.
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if err := flush(i); err != nil {
				return err
			}
		case c == byte(opUnion):
			if err := flush(i); err != nil {
				return err
			}
			if pendingOp {
				return missingOperand(expr, op, "right")
			}
			operand = false
		case c == byte(opDifference) || c == byte(opIntersection) || c == byte(opSymmetric):
			if err := flush(i); err != nil {
				return err
			}
			if pendingOp {
				return missingOperand(expr, op, "right")
			}
			if !operand {
				return missingOperand(expr, operator(c), "left")
			}
			op, operand, pendingOp = operator(c), false, true
		}
	}
	if depthBracket != 0 {
		return fmt.Errorf("unbalanced [ in %q", expr)
	}
	if err := flush(len(expr)); err != nil {
		return err
	}
	if pendingOp {
		return missingOperand(expr, op, "right")
	}
	return nil
}

// missingOperand reports an operator written without one of its operands.
func missingOperand(expr string, op operator, side string) error {
	return fmt.Errorf("in %q: the %c operator has no %s operand", expr, op, side)
}

// apply combines src into dst. src is used up.
func apply(dst *NodeSet, op operator, src *NodeSet) {
	switch op {
	case opDifference:
		dst.subtract(src)
	case opIntersection:
		dst.intersect(src)
	case opSymmetric:
		dst.symmetricDifference(src, true)
	default:
		dst.merge(src, true)
	}
}

// parseTerm parses a single term: either a group reference or a node pattern.
func parseTerm(term string, res Resolver, depth int, b *budget) (*NodeSet, error) {
	if strings.HasPrefix(term, "@") {
		return resolveGroup(term[1:], res, depth, b)
	}
	return parsePattern(term, b)
}

// parsePattern turns one node pattern into the set of the hosts it names.
func parsePattern(term string, b *budget) (*NodeSet, error) {
	g, err := parseProduct(term, b)
	if err != nil {
		return nil, err
	}
	return &NodeSet{groups: map[string]*group{g.pattern: g}}, nil
}

// parseProduct turns one node pattern into the product of its ranges,
// charging the hosts it names to b. Every dimension is weighed before it is
// expanded, against what the dimensions before it leave of the budget, so an
// oversized pattern is refused before its memory is spent.
func parseProduct(term string, b *budget) (*group, error) {
	// A host name never begins with a dash, and a name that does is read as
	// an option by ssh and most other tools a name is handed to.
	if strings.HasPrefix(term, "-") {
		return nil, fmt.Errorf("%q is not a host name: it begins with -", term)
	}
	var (
		pattern strings.Builder
		dims    []*rangeSet
		// Two numeric parts with nothing between them cannot be told apart
		// again once expanded: exe0[0,10] would print exe00, which reads as
		// a single number. Such a name is rejected rather than folded wrong.
		prevNumeric bool
		// total is the number of hosts the dimensions read so far name.
		total = 1
	)
	for i := 0; i < len(term); {
		c := term[i]
		numeric := c == '[' || (c >= '0' && c <= '9')
		if numeric && prevNumeric {
			return nil, fmt.Errorf("in %q: two numeric parts are adjacent at offset %d; separate them with a literal character", term, i)
		}
		prevNumeric = numeric

		switch {
		case c == '[':
			end := strings.IndexByte(term[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unbalanced [ in %q", term)
			}
			spans, count, err := parseSpans(term[i+1 : i+end])
			if err != nil {
				return nil, fmt.Errorf("in %q: %w", term, err)
			}
			if count > b.left/total {
				return nil, b.exceeded(term)
			}
			total *= count
			dims = append(dims, expandSpans(spans, count))
			pattern.WriteString("%s")
			i += end + 1
		case c >= '0' && c <= '9':
			j := i
			for j < len(term) && term[j] >= '0' && term[j] <= '9' {
				j++
			}
			lit := term[i:j]
			n, err := parseNumber(lit)
			if err != nil {
				return nil, fmt.Errorf("in %q: %w", term, err)
			}
			rs := &rangeSet{}
			rs.add(n, padOf(lit))
			dims = append(dims, rs)
			pattern.WriteString("%s")
			i = j
		case c == ']':
			return nil, fmt.Errorf("unbalanced ] in %q", term)
		case c == '%':
			pattern.WriteString("%%")
			i++
		default:
			pattern.WriteByte(c)
			i++
		}
	}
	if total > b.left {
		return nil, b.exceeded(term)
	}
	b.left -= total

	return newProduct(pattern.String(), dims), nil
}

// plainName is a name without brackets, one host, read into slices that
// are reused from name to name.
type plainName struct {
	pattern    []byte
	vals, pads []int
}

// parse reads a name without brackets and charges its host to b, as
// parsePattern reads and charges it.
func (p *plainName) parse(term string, b *budget) error {
	if err := p.read(term); err != nil {
		return err
	}
	if b.left < 1 {
		return b.exceeded(term)
	}
	b.left--
	return nil
}

// read reads a name without brackets into p.
func (p *plainName) read(term string) error {
	var err error
	p.pattern, p.vals, p.pads, err = readPlain(term, p.pattern[:0], p.vals[:0], p.pads[:0])
	return err
}

// readPlain reads a name without brackets, appending its pattern, its
// values and their widths to the slices given. It takes and returns the
// slices by value, so that buffers on a caller's stack stay there.
func readPlain(term string, pattern []byte, vals, pads []int) ([]byte, []int, []int, error) {
	if strings.HasPrefix(term, "-") {
		return nil, nil, nil, fmt.Errorf("%q is not a host name: it begins with -", term)
	}
	for i := 0; i < len(term); {
		c := term[i]
		switch {
		case c >= '0' && c <= '9':
			j := i
			for j < len(term) && term[j] >= '0' && term[j] <= '9' {
				j++
			}
			n, err := parseNumber(term[i:j])
			if err != nil {
				return nil, nil, nil, fmt.Errorf("in %q: %w", term, err)
			}
			vals = append(vals, n)
			pads = append(pads, normalPad(n, padOf(term[i:j])))
			pattern = append(pattern, "%s"...)
			i = j
		case c == '%':
			pattern = append(pattern, "%%"...)
			i++
		default:
			pattern = append(pattern, c)
			i++
		}
	}
	return pattern, vals, pads, nil
}
