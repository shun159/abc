package main

import (
	"go/ast"
	"go/types"
	"strings"
)

// A case of a switch on a value or a type, a dispatch, is read alone: the
// reader finds it, and reads it. One that does much is a function in all
// but name, and is measured alone too, by the case metric, to be moved
// out. It is measured as it is, not less for its repeats: statements
// alike in shape may each decide something else.

// A dispatchCase is a case of a dispatch, by its ABC size.
type dispatchCase struct {
	at      ast.Node
	label   string // its values, or default
	line    int
	a, b, c int
}

func (d dispatchCase) abc() float64 {
	return vec{a: float64(d.a), b: float64(d.b), c: float64(d.c)}.abc()
}

// keepCases keeps the cases of n, if a dispatch.
func (k *counter) keepCases(n ast.Node) {
	var clauses []ast.Stmt
	switch x := n.(type) {
	case *ast.SwitchStmt:
		if x.Tag == nil {
			return // an if else chain
		}
		clauses = x.Body.List
	case *ast.TypeSwitchStmt:
		clauses = x.Body.List
	default:
		return
	}
	for _, s := range clauses {
		cl := s.(*ast.CaseClause)
		d := dispatchCase{at: cl, label: label(cl)}
		for _, st := range cl.Body {
			ast.Inspect(st, func(x ast.Node) bool {
				d.a += assignments(x)
				d.b += branches(x)
				d.c += conditions(x)
				return true
			})
		}
		k.cases = append(k.cases, d)
	}
}

// label names the case cl by its values.
func label(cl *ast.CaseClause) string {
	if cl.List == nil {
		return "default"
	}
	var vs []string
	for _, e := range cl.List {
		vs = append(vs, types.ExprString(e))
	}
	return strings.Join(vs, ",")
}
