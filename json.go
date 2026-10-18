package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// The JSON of -json, for a model to act on: what -func explains, or the
// problems of a check.

// A jsonFunc is a function explained.
type jsonFunc struct {
	Pos   string     `json:"pos"`
	Func  string     `json:"func"`
	Sizes []jsonSize `json:"sizes"`
	Parts []jsonPart `json:"parts"`
}

// A jsonSize is a size of a function against its limit.
type jsonSize struct {
	Metric  string   `json:"metric"`
	Value   float64  `json:"value"`
	Limit   float64  `json:"limit"`
	Over    bool     `json:"over"`
	LeftOut bool     `json:"left_out,omitempty"` // by its directive
	ABC     *jsonABC `json:"abc,omitempty"`
}

type jsonABC struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
}

// A jsonPart is a statement or a case, by what it adds to its function.
type jsonPart struct {
	Line   int       `json:"line"`
	Depth  int       `json:"depth"`
	Code   string    `json:"code"`
	Size   float64   `json:"size"` // its ABC size
	ABC    jsonABC   `json:"abc"`
	Repeat float64   `json:"repeat,omitempty"` // how much is taken off, 0 to 1
	Like   int       `json:"like,omitempty"`   // the line of the sibling it repeats
	Case   *jsonCase `json:"case,omitempty"`   // if a case over the case metric
}

// A jsonCase is a case of a dispatch measured alone, as it is.
type jsonCase struct {
	Size  float64 `json:"size"`
	Limit float64 `json:"limit"`
}

// explainJSON writes what explainAll tells, as JSON.
func explainJSON(w io.Writer, scores []score, ms []metric, opts options) error {
	fs := []jsonFunc{}
	for _, s := range scores {
		if opts.focuses(s.key) {
			fs = append(fs, explained(s, ms))
		}
	}
	if len(fs) == 0 {
		return fmt.Errorf("no function %q", opts.focus)
	}
	return writeJSON(w, fs)
}

func explained(s score, ms []metric) jsonFunc {
	f := jsonFunc{Pos: s.pos, Func: s.key, Parts: []jsonPart{}}
	for _, m := range ms {
		if !m.perCase {
			f.Sizes = append(f.Sizes, sizeOf(s, m))
		}
	}
	long := longCases(s, ms)
	for _, p := range s.parts {
		c := p.counted()
		jp := jsonPart{Line: p.line, Depth: p.depth, Code: p.code, Size: c.abc(), ABC: abcJSON(c)}
		if p.like != nil {
			jp.Repeat, jp.Like = round(p.repeat*100)/100, p.likeLine
		}
		if d, ok := long[p.at]; ok {
			jp.Case = &jsonCase{d.abc(), limitOf(ms, "case")}
		}
		f.Parts = append(f.Parts, jp)
	}
	return f
}

func sizeOf(s score, m metric) jsonSize {
	v := m.of(s)
	left := slices.Contains(s.disabled, m.name)
	z := jsonSize{Metric: m.name, Value: v, Limit: m.limit, Over: !left && v > m.limit, LeftOut: left}
	if m.name == "abc" {
		abc := abcJSON(vec{a: s.a, b: s.b, c: s.c})
		z.ABC = &abc
	}
	return z
}

func abcJSON(v vec) jsonABC { return jsonABC{round(v.a), round(v.b), round(v.c)} }

// A jsonProblem is a size over its limit, or grown past its baseline.
type jsonProblem struct {
	Pos    string  `json:"pos"`
	Func   string  `json:"func"`
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Bound  float64 `json:"bound,omitempty"`
	Past   string  `json:"past"` // "limit", "baseline", or "unrecorded" if left out
	// LeftOut is of a size its directive leaves out: a hot path or a
	// critical section kept whole, not to split, but to abc -write.
	LeftOut bool   `json:"left_out,omitempty"`
	Case    string `json:"case,omitempty"` // the values of a case over the case metric
	Line    int    `json:"line,omitempty"` // and where it is
}

// A jsonReport is the problems of a check, and the baseline entries to
// lower.
type jsonReport struct {
	Problems []jsonProblem `json:"problems"`
	Shrunk   []string      `json:"shrunk,omitempty"`
}

// reportJSON writes the problems as JSON, and fails if any.
func reportJSON(w io.Writer, ps []problem, shrunk []string) error {
	r := jsonReport{Problems: []jsonProblem{}, Shrunk: shrunk}
	for _, p := range ps {
		past := map[string]string{pastLimit: "limit", pastBaseline: "baseline", unrecorded: "unrecorded"}[p.what]
		jp := jsonProblem{Pos: p.z.s.pos, Func: p.z.s.key, Metric: p.z.m.name, Value: p.z.value, Bound: p.bound, Past: past, LeftOut: p.z.leftOut}
		if p.z.cs != nil {
			jp.Case, jp.Line = p.z.cs.label, p.z.cs.line
		}
		r.Problems = append(r.Problems, jp)
	}
	if err := writeJSON(w, r); err != nil {
		return err
	}
	if len(ps) > 0 {
		return fmt.Errorf("%d sizes too large", len(ps))
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

// A jsonChange is what -diff tells, for a model: each function the change
// touched, and the sum, to tell a split that simplifies from one that
// spreads.
type jsonChange struct {
	Against   string        `json:"against"`
	Functions []jsonChanged `json:"functions"`
	Largest   [2]float64    `json:"largest"` // ABC size, before and after
	Total     [2]float64    `json:"total"`
	Count     [2]float64    `json:"count"`
}

type jsonChanged struct {
	Func   string     `json:"func"`
	Before *jsonSizes `json:"before,omitempty"` // absent if new
	After  *jsonSizes `json:"after,omitempty"`  // absent if gone
	Split  *jsonSplit `json:"split,omitempty"`  // of a new function
}

type jsonSizes struct {
	Pos     string  `json:"pos"`
	ABC     float64 `json:"abc"`
	Lines   float64 `json:"lines"`
	Cyclo   float64 `json:"cyclo"`
	Nesting int     `json:"nesting"`
}

type jsonSplit struct {
	Calls       int  `json:"calls"`
	Statements  int  `json:"statements"`
	PassThrough bool `json:"pass_through"`
}

func diffJSON(rev string, cs []changed) jsonChange {
	sum := summarize(cs)
	j := jsonChange{Against: rev, Functions: []jsonChanged{}, Largest: sum.largest, Count: sum.count,
		Total: [2]float64{round(sum.total[0]), round(sum.total[1])}}
	for _, c := range cs {
		jc := jsonChanged{Func: c.key, Before: sizesOf(c.before), After: sizesOf(c.after)}
		if c.split != nil {
			jc.Split = &jsonSplit{c.split.calls, c.split.statements, c.split.passThrough}
		}
		j.Functions = append(j.Functions, jc)
	}
	return j
}

func sizesOf(v *version) *jsonSizes {
	if v == nil {
		return nil
	}
	return &jsonSizes{v.s.pos, v.s.abc(), v.s.lines, v.s.cyclo, v.s.nesting}
}
