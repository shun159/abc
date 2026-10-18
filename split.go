package main

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
)

// A change that only spreads code makes functions that hide little:
// called at one place, of a statement, or passing their arguments on to
// another function (Ousterhout's pass-through methods). Read alone, such
// a function is easy; read with its caller, it is a jump more. These are
// the signs of it, of each function a change added, told, not failed.

// splitSigns are the signs of a split too far, of a function added.
type splitSigns struct {
	calls       int  // the places that call it, in its package
	statements  int  // of its body
	passThrough bool // it calls another with its arguments, and no more
}

// describe tells the signs s, if any.
func (s *splitSigns) describe() string {
	if s == nil {
		return ""
	}
	parts := []string{"new"}
	switch s.calls {
	case 0:
		parts = append(parts, "not called")
	case 1:
		parts = append(parts, "called at one place")
	}
	if s.statements <= 1 {
		parts = append(parts, "of one statement")
	}
	if s.passThrough {
		parts = append(parts, "passing through")
	}
	return "  (" + strings.Join(parts, ", ") + ")"
}

// A callCounter counts the calls of each name in the package of a
// directory, as it is now, by directory.
type callCounter map[string]map[string]int

// signsOf returns the signs of a split of v, a function added.
func (cc callCounter) signsOf(v *version) (splitSigns, error) {
	counts, err := cc.of(filepath.Dir(v.path))
	if err != nil {
		return splitSigns{}, err
	}
	return splitSigns{
		calls:       counts[v.decl.Name.Name],
		statements:  len(v.decl.Body.List),
		passThrough: passesThrough(v.decl),
	}, nil
}

// of counts the calls of each name in the files of dir, but tests: of a
// function, or of a method of any type.
func (cc callCounter) of(dir string) (map[string]int, error) {
	if counts, ok := cc[dir]; ok {
		return counts, nil
	}
	counts := map[string]int{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !isSource(e.Name()) {
			continue
		}
		_, f, _, err := parse(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		countCalls(f, counts)
	}
	cc[dir] = counts
	return counts, nil
}

func countCalls(f *ast.File, counts map[string]int) {
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch fun := c.Fun.(type) {
			case *ast.Ident:
				counts[fun.Name]++
			case *ast.SelectorExpr:
				counts[fun.Sel.Name]++
			}
		}
		return true
	})
}

// passesThrough tells whether the body of fn is a call alone, returned or
// not, of its parameters and its receiver, and constants.
func passesThrough(fn *ast.FuncDecl) bool {
	if len(fn.Body.List) != 1 {
		return false
	}
	call := soleCall(fn.Body.List[0])
	if call == nil {
		return false
	}
	own := paramNames(fn)
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *ast.Ident:
			if !own[a.Name] {
				return false
			}
		case *ast.BasicLit:
		default:
			return false
		}
	}
	return true
}

// soleCall is the call that st is, or returns alone, if it is one.
func soleCall(st ast.Stmt) *ast.CallExpr {
	var e ast.Expr
	switch s := st.(type) {
	case *ast.ExprStmt:
		e = s.X
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return nil
		}
		e = s.Results[0]
	}
	call, _ := e.(*ast.CallExpr)
	return call
}

// paramNames are the names of the parameters and the receiver of fn.
func paramNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	lists := []*ast.FieldList{fn.Type.Params, fn.Recv}
	for _, l := range lists {
		if l == nil {
			continue
		}
		for _, f := range l.List {
			for _, n := range f.Names {
				names[n.Name] = true
			}
		}
	}
	return names
}
