package main

import (
	"go/ast"
	"math"
	"reflect"
)

// Regular code reads faster than it counts: once the reader knows the
// first of statements alike, the next ones are read as repeats of it
// (Jbara and Feitelson, "How programmers read regular code", EMSE 2017).
// So in every list of siblings, the statements of a block, the cases of a
// switch or a select, or the elements of a literal, a sibling counts as
// much as it is new: all of it for the first, and less for one more like a
// sibling before it.
//
// Alike is by shape: the kinds of the nodes in order, with identifiers and
// literals but of a kind, so that m.Term = d.uint() and m.To = d.pid() are
// alike, and a case of a call like the case before it; but only siblings
// of a kind, two ifs or two assignments. A likeness up to the floor is of
// any code, and discounts nothing.

// A vec is what some code counts: its assignments, branches, conditions,
// paths and lines, each less for a repeat.
type vec struct{ a, b, c, paths, lines float64 }

func (v vec) add(w vec) vec {
	return vec{v.a + w.a, v.b + w.b, v.c + w.c, v.paths + w.paths, v.lines + w.lines}
}

func (v vec) scale(k float64) vec {
	return vec{v.a * k, v.b * k, v.c * k, v.paths * k, v.lines * k}
}

func (v vec) abc() float64 { return round(math.Sqrt(v.a*v.a + v.b*v.b + v.c*v.c)) }

// A counter counts code, with floor the likeness up to which siblings are
// not repeats, span the lines of a node, but blank ones and comments, and
// keeps the cases of its dispatches.
type counter struct {
	floor float64
	span  func(ast.Node) float64
	cases []dispatchCase

	explain bool   // keep the parts, to tell
	parts   []part // the siblings counted, in order, if explain
	depth   int    // of the siblings counted now
}

// A part is a sibling counted, to tell where a function's size is.
type part struct {
	at     ast.Node
	depth  int
	whole  vec      // all of it, its own parts less for their repeats
	repeat float64  // how much of it is a repeat
	like   ast.Node // the sibling before it most like it, if any

	line, likeLine int    // where it and its like are
	code           string // its first line, from where it starts
}

// counted is what p adds to its function.
func (p part) counted() vec { return p.whole.scale(1 - p.repeat) }

// list counts siblings, each as much as it is new, and its lines.
func (k *counter) list(nodes []ast.Node) vec { return k.siblings(nodes, true) }

// elems counts the elements of a literal as siblings, each as much as it
// is new; their lines are of the statement that holds the literal, many
// elements may share one.
func (k *counter) elems(nodes []ast.Node) vec { return k.siblings(nodes, false) }

// siblings counts nodes, each as much as it is new, with its lines if
// lines.
func (k *counter) siblings(nodes []ast.Node, lines bool) vec {
	var total vec
	var shapes [][]string
	for _, n := range nodes {
		sh := shape(n)
		alike, like := 0.0, -1
		for i, before := range shapes {
			if l := likeness(sh, before); l > alike {
				alike, like = l, i
			}
		}
		shapes = append(shapes, sh)
		p := k.count(n, lines)
		p.repeat = k.repeat(alike)
		if like >= 0 && p.repeat > 0 {
			p.like = nodes[like]
		}
		k.keep(p)
		total = total.add(p.counted())
	}
	return total
}

// count counts the sibling n, its lines with it if lines, and keeps a
// place for it before its own parts, if explain.
func (k *counter) count(n ast.Node, lines bool) part {
	p := part{at: n, depth: k.depth}
	if k.explain {
		k.parts = append(k.parts, p)
	}
	k.depth++
	p.whole = k.node(n)
	k.depth--
	if lines {
		p.whole.lines += k.span(n)
	}
	return p
}

// keep keeps p in the place count kept for it.
func (k *counter) keep(p part) {
	if !k.explain {
		return
	}
	for i := len(k.parts) - 1; i >= 0; i-- {
		if k.parts[i].at == p.at {
			k.parts[i] = p
			return
		}
	}
}

// repeat is how much of a sibling is a repeat, of its likeness to the most
// like before it.
func (k *counter) repeat(alike float64) float64 {
	if alike <= k.floor {
		return 0
	}
	return (alike - k.floor) / (1 - k.floor)
}

// node counts n, its lists of siblings as lists: their lines are theirs,
// not n's.
func (k *counter) node(n ast.Node) vec {
	var v vec
	ast.Inspect(n, func(x ast.Node) bool {
		if x == nil {
			return true
		}
		v = v.add(own(x))
		k.keepCases(x)
		lists, holds := k.lists(x)
		v = v.add(lists)
		return !holds
	})
	return v
}

// own is what x counts by itself, not by its parts.
func own(x ast.Node) vec {
	return vec{
		a:     float64(assignments(x)),
		b:     float64(branches(x)),
		c:     float64(conditions(x)),
		paths: float64(paths(x)),
	}
}

// lists counts the parts of x, if it holds a list of siblings: a block,
// a case and what it is for, or a literal and its elements.
func (k *counter) lists(x ast.Node) (v vec, holds bool) {
	switch c := x.(type) {
	case *ast.BlockStmt:
		return k.inner(stmts(c.List)), true
	case *ast.CaseClause:
		for _, e := range c.List {
			v = v.add(k.node(e))
		}
		return v.add(k.inner(stmts(c.Body))), true
	case *ast.CommClause:
		if c.Comm != nil {
			v = k.node(c.Comm)
		}
		return v.add(k.inner(stmts(c.Body))), true
	case *ast.CompositeLit:
		if c.Type != nil {
			v = k.node(c.Type)
		}
		return v.add(k.elems(exprs(c.Elts))), true
	}
	return vec{}, false
}

// inner counts the list nodes in a node, which spans their lines.
func (k *counter) inner(nodes []ast.Node) vec {
	v := k.list(nodes)
	for _, n := range nodes {
		v.lines -= k.span(n)
	}
	return v
}

func exprs(es []ast.Expr) []ast.Node {
	ns := make([]ast.Node, len(es))
	for i, e := range es {
		ns[i] = e
	}
	return ns
}

func stmts(ss []ast.Stmt) []ast.Node {
	ns := make([]ast.Node, len(ss))
	for i, s := range ss {
		ns[i] = s
	}
	return ns
}

// shape is n in order, by the kinds of its nodes, a ) closing each:
// identifiers and literals by their kind alone, operators by theirs.
func shape(n ast.Node) []string {
	var out []string
	ast.Inspect(n, func(x ast.Node) bool {
		if x == nil {
			out = append(out, ")")
			return true
		}
		out = append(out, kind(x))
		return true
	})
	return out
}

func kind(x ast.Node) string {
	switch v := x.(type) {
	case *ast.Ident:
		return "id"
	case *ast.BasicLit:
		return "lit:" + v.Kind.String()
	case *ast.BinaryExpr:
		return "bin" + v.Op.String()
	case *ast.UnaryExpr:
		return "un" + v.Op.String()
	case *ast.AssignStmt:
		return "assign" + v.Tok.String()
	}
	return reflect.TypeOf(x).Elem().Name()
}

// maxShape is the longest shape compared: longer code is not a repeat of
// a line or two, and its likeness costs the square of its length.
const maxShape = 600

// likeness is 1 less the edit distance of a and b over the longer: 1 for
// alike, 0 for nothing in common. Code of other kinds, an if and a for, is
// no repeat, however its parts are alike.
func likeness(a, b []string) float64 {
	if len(a) > maxShape || len(b) > maxShape {
		return 0
	}
	if len(a) == 0 || len(b) == 0 || a[0] != b[0] {
		return 0
	}
	return 1 - float64(distance(a, b))/float64(max(len(a), len(b)))
}

// distance is the edit distance of a and b.
func distance(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			sub := prev[j-1]
			if a[i-1] != b[j-1] {
				sub++
			}
			cur[j] = min(sub, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
