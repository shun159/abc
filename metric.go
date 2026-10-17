package main

import (
	"fmt"
	"slices"
)

// A metric is one measure of a function, or of each case of its
// dispatches, and the largest it may be.
type metric struct {
	name    string
	limit   float64
	of      func(score) float64
	perCase bool
}

func metrics(abc float64, lines, nesting, cyclo int, longCase float64) []metric {
	return []metric{
		{"abc", abc, score.abc, false},
		{"lines", float64(lines), func(s score) float64 { return s.lines }, false},
		{"nesting", float64(nesting), func(s score) float64 { return float64(s.nesting) }, false},
		{"cyclo", float64(cyclo), func(s score) float64 { return s.cyclo }, false},
		{"case", longCase, nil, true},
	}
}

func names(ms []metric) []string {
	var ns []string
	for _, m := range ms {
		ns = append(ns, m.name)
	}
	return ns
}

// A size is a function, or a case of it, by one metric.
type size struct {
	s     score
	m     metric
	key   string // its key in the baseline
	value float64
	cs    *dispatchCase // the case, of the case metric

	// leftOut is of a metric its function's directive leaves out: of no
	// limit, but kept in the baseline, not to grow.
	leftOut bool
}

// sizes measures each of the scores by each of the metrics, those its
// directive leaves out too.
func sizes(scores []score, ms []metric) []size {
	var zs []size
	for _, s := range scores {
		for _, m := range ms {
			var ofM []size
			if m.perCase {
				ofM = caseSizes(s, m)
			} else {
				ofM = []size{{s: s, m: m, key: baselineKey(s, m), value: m.of(s)}}
			}
			for i := range ofM {
				ofM[i].leftOut = slices.Contains(s.disabled, m.name)
			}
			zs = append(zs, ofM...)
		}
	}
	return zs
}

// caseSizes measures each case of the dispatches of s, by the case metric
// m: its ABC size, keyed by its values, which move less than its line.
func caseSizes(s score, m metric) []size {
	var zs []size
	for i := range s.cases {
		cs := &s.cases[i]
		zs = append(zs, size{s: s, m: m, key: s.key + " case " + cs.label, value: cs.abc(), cs: cs})
	}
	return zs
}

// baselineKey is the function, and the metric but the ABC size: the
// baseline was of it alone.
func baselineKey(s score, m metric) string {
	if m.name == "abc" {
		return s.key
	}
	return s.key + " " + m.name
}

func (z size) over() bool { return z.value > z.m.limit }

// kept tells whether z is in the baseline: over its limit, or left out.
func (z size) kept() bool { return z.over() || z.leftOut }

// ratio is how far over its limit z is, to compare sizes of metrics of
// other units.
func (z size) ratio() float64 {
	if z.m.limit <= 0 {
		return z.value
	}
	return z.value / z.m.limit
}

// String describes z, with the assignments, branches and conditions of
// an ABC size.
func (z size) String() string {
	return fmt.Sprintf("%s %s", z.s.key, z.measured())
}

// measured is the metric and value of z, with the assignments, branches
// and conditions of an ABC size.
func (z size) measured() string {
	switch {
	case z.cs != nil:
		return fmt.Sprintf("case %s (line %d) abc %g (A%d B%d C%d)", z.cs.label, z.cs.line, z.value, z.cs.a, z.cs.b, z.cs.c)
	case z.m.name == "abc":
		return fmt.Sprintf("abc %g (A%g B%g C%g)", z.value, z.s.a, z.s.b, z.s.c)
	}
	return fmt.Sprintf("%s %g", z.m.name, z.value)
}
