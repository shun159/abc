package main

import (
	"go/ast"
	"go/token"
	"math"
	"strings"
)

// A score is the ABC size of a function: its assignments, its branches
// (calls), and its conditions, as RuboCop's Metrics/AbcSize counts them,
// adapted to Go, each less for a repeat.
type score struct {
	key     string // the directory and name of the function
	pos     string // where it is
	a, b, c float64
	lines   float64 // of its body, but blank ones and comments
	nesting int     // the deepest of its ifs, fors, switches and selects
	cyclo   float64 // its cyclomatic complexity

	cases    []dispatchCase // the cases of its dispatches
	parts    []part         // where its size is, if explained
	disabled []string       // the metrics left out by its directive
}

// abc is the ABC size of s.
func (s score) abc() float64 { return vec{a: s.a, b: s.b, c: s.c}.abc() }

// round rounds to a tenth, as sizes are printed and compared.
func round(x float64) float64 { return math.Round(x*10) / 10 }

// measure counts the assignments, calls, conditions, paths and lines of
// the body of fn, each less for a repeat, with floor the likeness up to
// which siblings are not repeats and span the lines of a node; and its
// nesting, and the cases of its dispatches.
func measure(fn *ast.FuncDecl, floor float64, span func(ast.Node) float64, explain bool) score {
	k := counter{floor: floor, span: span, explain: explain}
	v := k.list(stmts(fn.Body.List))
	return score{
		a: round(v.a), b: round(v.b), c: round(v.c),
		cyclo: round(1 + v.paths), lines: round(v.lines),
		nesting: nesting(fn.Body),
		cases:   k.cases, parts: k.parts,
	}
}

// spanOf returns the lines of a node of the file fset of lines src, but
// blank ones and comments.
func spanOf(fset *token.FileSet, src []string) func(ast.Node) float64 {
	return func(n ast.Node) float64 {
		return float64(lines(src, fset.Position(n.Pos()).Line, fset.Position(n.End()).Line))
	}
}

// lines counts the lines of src from first to last, but blank ones and
// comments.
func lines(src []string, first, last int) int {
	n := 0
	for i := first; i <= last; i++ {
		line := strings.TrimSpace(src[i-1])
		if line != "" && !strings.HasPrefix(line, "//") {
			n++
		}
	}
	return n
}

// nesting is the deepest nesting of ifs, fors, switches and selects under
// n. An else if is as deep as its if; a func literal does not nest.
func nesting(n ast.Node) int {
	deepest := 0
	ast.Inspect(n, func(m ast.Node) bool {
		switch {
		case m == n || !nests(m):
			return true
		case elseIf(n, m):
			deepest = max(deepest, nesting(m))
		default:
			deepest = max(deepest, 1+nesting(m))
		}
		return false
	})
	return deepest
}

func nests(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

func elseIf(parent, n ast.Node) bool {
	p, ok := parent.(*ast.IfStmt)
	return ok && p.Else == n
}

// paths counts the paths n adds, as gocyclo does: an if, a for, a case but
// the default, a && or ||.
func paths(n ast.Node) int {
	switch x := n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return 1
	case *ast.CaseClause:
		if x.List != nil {
			return 1
		}
	case *ast.CommClause:
		if x.Comm != nil {
			return 1
		}
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			return 1
		}
	}
	return 0
}

func assignments(n ast.Node) int {
	switch x := n.(type) {
	case *ast.AssignStmt:
		return len(x.Lhs)
	case *ast.IncDecStmt:
		return 1
	case *ast.ValueSpec:
		return len(x.Names)
	}
	return 0
}

func branches(n ast.Node) int {
	if _, ok := n.(*ast.CallExpr); ok {
		return 1
	}
	return 0
}

// conditions counts an if, and its else; a case; a comparison, && and ||;
// a !.
func conditions(n ast.Node) int {
	switch x := n.(type) {
	case *ast.IfStmt:
		return 1 + elseOf(x)
	case *ast.CaseClause, *ast.CommClause:
		return 1
	case *ast.BinaryExpr:
		return conditional(x.Op)
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return 1
		}
	}
	return 0
}

// elseOf counts the else of x, not an else if, which counts as an if.
func elseOf(x *ast.IfStmt) int {
	if x.Else == nil {
		return 0
	}
	if _, elseIf := x.Else.(*ast.IfStmt); elseIf {
		return 0
	}
	return 1
}

func conditional(op token.Token) int {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ, token.LAND, token.LOR:
		return 1
	}
	return 0
}

// name is the name of fn, after the type of its receiver, if a method.
func name(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return typeName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func typeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return typeName(t.X)
	case *ast.IndexExpr:
		return typeName(t.X)
	case *ast.IndexListExpr:
		return typeName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}
