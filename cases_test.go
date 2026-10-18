package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseFunc(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\n"+src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return f.Decls[0].(*ast.FuncDecl)
}

func measureSrc(t *testing.T, src string) score {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.go")
	if err := os.WriteFile(path, []byte("package x\n\n"+src), 0o644); err != nil {
		t.Fatal(err)
	}
	scores, err := scanFile(path, []string{"abc", "lines", "nesting", "cyclo", "case"}, options{floor: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	return scores[0]
}

func TestARepeatCountsNothing(t *testing.T) {
	s := measureSrc(t, `func f(m *msg, d *decoder) {
	m.Term = d.uint()
	m.Leader = d.pid()
	m.Index = d.uint()
	m.Size = d.int()
}`)
	// The first; the rest are alike.
	if s.a != 1 || s.b != 1 || s.c != 0 || s.lines != 1 {
		t.Errorf("A%g B%g C%g, lines %g; want A1 B1 C0, 1", s.a, s.b, s.c, s.lines)
	}
}

func TestStatementsUnlikeCountAll(t *testing.T) {
	s := measureSrc(t, `func f(k int) int {
	x := g(k)
	if x > k {
		return x
	}
	for range k {
		x++
	}
	return x
}`)
	if s.a != 2 || s.b != 1 || s.c != 2 || s.cyclo != 3 || s.lines != 8 {
		t.Errorf("A%g B%g C%g, cyclo %g, lines %g; want A2 B1 C2, 3, 8", s.a, s.b, s.c, s.cyclo, s.lines)
	}
}

func TestADispatchOfCasesAlikeIsSmall(t *testing.T) {
	s := measureSrc(t, `func f(r *runner, e any) {
	switch e := e.(type) {
	case send:
		r.send(e.To, e.Msg)
	case link:
		r.env.Link(e.PID)
	case unlink:
		r.env.Unlink(e.PID)
	case trap:
		r.env.TrapExit(e.On)
	case monitor:
		r.env.MonitorNodes(e.On)
	}
}`)
	// As if of two or three cases: the first, and less of the rest.
	if s.abc() > 4 || s.cyclo > 4 || s.lines > 6 {
		t.Errorf("abc %g, cyclo %g, lines %g; want at most 4, 4, 6", s.abc(), s.cyclo, s.lines)
	}
	if len(s.cases) != 5 {
		t.Errorf("%d cases; want 5", len(s.cases))
	}
}

func TestACaseIsMeasuredAsItIs(t *testing.T) {
	s := measureSrc(t, `func f(v value, b *builder) {
	switch v.Kind() {
	case pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		if v.Type().Elem() == locationType {
			fmt.Fprintf(b, "loc%x", v.Pointer())
			return
		}
		if n, ok := seen[v.Pointer()]; ok {
			fmt.Fprintf(b, "@%d", n)
			return
		}
	default:
		b.WriteString("?")
	}
}`)
	// Its ifs are alike in shape, but it is measured as it is.
	c := s.cases[0]
	if c.label != "pointer" || c.a != 2 || c.b != 8 || c.c != 4 {
		t.Errorf("%s A%d B%d C%d; want pointer A2 B8 C4", c.label, c.a, c.b, c.c)
	}
}

func TestATaglessSwitchHasNoCases(t *testing.T) {
	s := measureSrc(t, `func f(k int) {
	switch {
	case k < 0:
		a()
	case k > 0:
		b()
	}
}`)
	if len(s.cases) != 0 {
		t.Errorf("%d cases; want none", len(s.cases))
	}
}

func TestDisable(t *testing.T) {
	known := []string{"abc", "lines", "nesting", "cyclo", "case"}
	for _, tc := range []struct {
		doc  string
		want []string
		bad  bool
	}{
		{"//abc:disable abc,lines the hot path of every write", []string{"abc", "lines"}, false},
		{"//abc:disable abc", nil, true},
		{"//abc:disable size the path", nil, true},
		{"//abc:dispatch one case an op", nil, true},
	} {
		fn := parseFunc(t, tc.doc+"\nfunc f() {}")
		got, err := disabled(fn, known)
		if (err != nil) != tc.bad || len(got) != len(tc.want) {
			t.Errorf("%s: %v, %v; want %v, error %t", tc.doc, got, err, tc.want, tc.bad)
		}
	}
}

func TestExplain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.go")
	src := "package x\n\nfunc f(m *msg, d *decoder) {\n\tm.Term = d.uint()\n\tm.Index = d.uint()\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := options{floor: 0.3, focus: "f"}
	scores, err := scanFile(path, []string{"abc"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := explainAll(&b, scores, metrics(17, 40, 3, 10, 10), opts); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"m.Term = d.uint()", "100% of 4", "m.Index = d.uint()"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("no %q in\n%s", want, b.String())
		}
	}
	if err := explainAll(&b, scores, nil, options{focus: "g"}); err == nil {
		t.Error("explained a function that is not there")
	}
}

func TestTheFieldsOfALiteralAlikeCountLess(t *testing.T) {
	s := measureSrc(t, `func f(seed uint64) *sim {
	return &sim{
		nodes:   make(map[string]*node),
		cuts:    make(map[[2]string]bool),
		held:    make(map[[2]string]bool),
		paused:  make(map[string]bool),
		byPID:   make(map[pid]*process),
		aliases: make(map[ref]*alias),
	}
}`)
	// Six makes; the first counts, the rest less as they repeat it, of
	// other types of maps. The lines are the statement's.
	if s.b >= 3 || s.lines != 8 {
		t.Errorf("B%g, lines %g; want B under 3, of 6 calls, and 8 lines", s.b, s.lines)
	}
}
