package main

import (
	"fmt"
	"go/ast"
	"io"
	"slices"
	"strings"
)

// explainAll tells, of each function opts focuses on, its sizes against
// their limits, and where they are: each statement and case, by what it
// adds, less for a repeat of a sibling before it.
func explainAll(w io.Writer, scores []score, ms []metric, opts options) error {
	found := false
	for _, s := range scores {
		if opts.focuses(s.key) {
			found = true
			explain(w, s, ms)
		}
	}
	if !found {
		return fmt.Errorf("no function %q", opts.focus)
	}
	return nil
}

func explain(w io.Writer, s score, ms []metric) {
	fmt.Fprintf(w, "%s: %s\n  %s\n\n", s.pos, s.key, summary(s, ms))
	fmt.Fprintln(w, "  abc is what a part adds to its function, its own parts in it; the parts")
	fmt.Fprintln(w, "  under one are shown as they add to it, before its repeat is taken off.")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %5s %5s  %-20s %-13s %s\n", "line", "abc", "A B C", "repeat", "code")
	long := longCases(s, ms)
	for _, p := range s.parts {
		c := p.counted()
		note := ""
		if p.like != nil {
			note = fmt.Sprintf("%3.0f%% of %d", 100*p.repeat, p.likeLine)
		}
		code := strings.Repeat("  ", p.depth) + p.code
		if d, ok := long[p.at]; ok {
			code += fmt.Sprintf("    <- case abc %g > %g, as it is", d.abc(), limitOf(ms, "case"))
		}
		fmt.Fprintf(w, "  %5d %5.1f  %-20s %-13s %s\n", p.line, c.abc(), abcOf(c), note, code)
	}
	fmt.Fprintln(w)
}

// summary is the sizes of s, each with its limit if over it, or left out.
func summary(s score, ms []metric) string {
	var parts []string
	for _, m := range ms {
		if m.perCase {
			continue
		}
		v := m.of(s)
		part := fmt.Sprintf("%s %g", m.name, v)
		if m.name == "abc" {
			part += " (" + abcOf(vec{a: s.a, b: s.b, c: s.c}) + ")"
		}
		switch {
		case slices.Contains(s.disabled, m.name):
			part += " (left out)"
		case v > m.limit:
			part += fmt.Sprintf(" > %g", m.limit)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// longCases are the cases of s over the case metric, by their clause.
func longCases(s score, ms []metric) map[ast.Node]dispatchCase {
	long := map[ast.Node]dispatchCase{}
	if slices.Contains(s.disabled, "case") {
		return long
	}
	for _, d := range s.cases {
		if d.abc() > limitOf(ms, "case") {
			long[d.at] = d
		}
	}
	return long
}

func limitOf(ms []metric, name string) float64 {
	for _, m := range ms {
		if m.name == name {
			return m.limit
		}
	}
	return 0
}

func abcOf(v vec) string {
	return fmt.Sprintf("A%g B%g C%g", round(v.a), round(v.b), round(v.c))
}
