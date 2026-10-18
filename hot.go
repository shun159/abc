package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A hot path, code every operation runs, is marked as such, with the
// benchmarks that measure it:
//
//	//abc:hot BenchmarkPut,BenchmarkMixed the path of every write
//
// A benchmark of another package is named after its directory, from where
// abc runs: //abc:hot pmap/BenchmarkGet,kvs/BenchmarkGet.
//
// It leaves nothing out: it tells that a change to it is to be measured,
// and by what. abc -hot checks that its benchmarks are there, and lists
// the calls in it the compiler does not inline: a function of the module
// over the inliner's budget, or one it could inline and did not there, as
// a function of another package in a generic body. Those are what a split
// of a hot path most often costs.
const hotDirective = "//abc:hot"

// hotOf returns the benchmarks the hot directive on fn names, if any.
func hotOf(fn *ast.FuncDecl) []string {
	if fn.Doc == nil {
		return nil
	}
	for _, c := range fn.Doc.List {
		if rest, found := strings.CutPrefix(c.Text, hotDirective+" "); found {
			list, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
			return strings.Split(list, ",")
		}
	}
	return nil
}

// A hotFunc is a function marked hot, where it is, and what of it to tell.
type hotFunc struct {
	key, pos   string
	benchmarks []string
	missing    []string   // benchmarks not in its package
	slow       []slowCall // calls not inlined
	compiled   bool       // told of by the compiler: a generic body is compiled only instantiated
	fn         *ast.FuncDecl
	fset       *token.FileSet
	dir        string
}

// A slowCall is a call in a hot function the compiler does not inline.
type slowCall struct {
	line   int
	callee string
	why    string // the compiler's reason, if it gave one
}

// runHot tells, of each function marked hot under the patterns, its
// benchmarks and its calls not inlined; it fails if a benchmark is not
// there.
func runHot(patterns []string, w io.Writer) error {
	hs, err := findHot(patterns)
	if err != nil {
		return err
	}
	missing := 0
	byDir := map[string][]*hotFunc{}
	for _, h := range hs {
		h.missing = missingBenchmarks(h.dir, h.benchmarks)
		missing += len(h.missing)
		byDir[h.dir] = append(byDir[h.dir], h)
	}
	for dir, hs := range byDir {
		if err := inlined(dir, hs); err != nil {
			return err
		}
	}
	for _, h := range hs {
		h.report(w)
	}
	if missing > 0 {
		return fmt.Errorf("%d benchmarks of hot paths not found", missing)
	}
	return nil
}

// findHot finds the functions marked hot in the Go files under the
// patterns, but tests.
func findHot(patterns []string) ([]*hotFunc, error) {
	var hs []*hotFunc
	for _, p := range patterns {
		dir, recursive := strings.CutSuffix(p, "/...")
		files, err := goFiles(dir, recursive)
		if err != nil {
			return nil, err
		}
		for _, path := range files {
			found, err := hotIn(path)
			if err != nil {
				return nil, err
			}
			hs = append(hs, found...)
		}
	}
	sort.Slice(hs, func(i, j int) bool { return hs[i].pos < hs[j].pos })
	return hs, nil
}

// hotIn returns the functions marked hot in the file path.
func hotIn(path string) ([]*hotFunc, error) {
	fset, f, _, err := parse(path)
	if err != nil {
		return nil, err
	}
	var hs []*hotFunc
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if bs := hotOf(fn); bs != nil {
			key := filepath.ToSlash(filepath.Dir(path)) + " " + name(fn)
			hs = append(hs, &hotFunc{key: key, pos: fset.Position(fn.Pos()).String(),
				benchmarks: bs, fn: fn, fset: fset, dir: filepath.Dir(path)})
		}
	}
	return hs, nil
}

// missingBenchmarks returns the benchmarks of bs not declared in the tests
// of their package: dir's, or the one named before the benchmark, as
// pmap/BenchmarkGet, from the directory abc runs in.
func missingBenchmarks(dir string, bs []string) []string {
	var missing []string
	for _, b := range bs {
		in, name := dir, b
		if i := strings.LastIndex(b, "/"); i >= 0 {
			in, name = filepath.FromSlash(b[:i]), b[i+1:]
		}
		if !benchmarksOf(in)[name] {
			missing = append(missing, b)
		}
	}
	return missing
}

// benchmarksOf returns the functions declared in the tests of dir.
func benchmarksOf(dir string) map[string]bool {
	declared := map[string]bool{}
	tests, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	for _, t := range tests {
		_, f, _, err := parse(t)
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
				declared[fn.Name.Name] = true
			}
		}
	}
	return declared
}

// compilerLine is a line of go build -gcflags=-m=2: where, and what.
var compilerLine = regexp.MustCompile(`^(.+?):(\d+):(\d+): (inlining call to|cannot inline|can inline) (\S+)(.*)$`)

// lastName is the last identifier of what the compiler names a function:
// pkg.(*T[shape]).m is m.
var lastName = regexp.MustCompile(`([A-Za-z_]\w*)$`)

// inlined compiles the package of dir with its tests, which instantiate
// its generic code, asking the compiler what it inlines, and finds the
// calls of the hot functions hs it does not.
func inlined(dir string, hs []*hotFunc) error {
	out, err := compilerSays(dir)
	if err != nil {
		return err
	}
	inl := readInlining(out)
	for _, h := range hs {
		decl := h.fset.Position(h.fn.Pos())
		h.compiled = inl.judged[place(decl.Filename, fmt.Sprint(decl.Line), "")]
		if h.compiled {
			h.slow = slowCalls(h, inl)
		}
	}
	return nil
}

// compilerSays compiles the package of dir with its tests, and returns
// what the compiler told of its inlining.
func compilerSays(dir string) (*bytes.Buffer, error) {
	var out bytes.Buffer
	cmd := exec.Command("go", "test", "-c", "-o", os.DevNull, "-gcflags=-m=2", "./"+filepath.ToSlash(dir))
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil && !bytes.Contains(out.Bytes(), []byte("inline")) {
		return nil, fmt.Errorf("go test -c %s: %w\n%s", dir, err, out.Bytes())
	}
	return &out, nil
}

// inlining is what the compiler told: the calls it inlined, by place, the
// functions of the module it judged, by name and by the line of their
// declaration, and why it cannot inline those it cannot.
type inlining struct {
	sites, own, judged map[string]bool
	why                map[string]string
}

// addWhy adds why a function of the name base cannot be inlined: the
// calls are matched by name, so of functions of one name, each reason.
func (inl inlining) addWhy(base, why string) {
	switch had := inl.why[base]; {
	case had == "":
		inl.why[base] = why
	case !strings.Contains(had, why):
		inl.why[base] = had + "; or, of another " + base + ", " + why
	}
}

// readInlining reads the compiler's lines.
func readInlining(r io.Reader) inlining {
	inl := inlining{map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if m := compilerLine.FindStringSubmatch(sc.Text()); m != nil {
			inl.read(m)
		}
	}
	return inl
}

// read reads a line of the compiler, as compilerLine matched it: a call
// inlined, or a function of the module it can or cannot inline.
func (inl inlining) read(m []string) {
	path, line, verb, rest := m[1], m[2], m[4], m[6]
	base := lastName.FindString(strings.TrimSuffix(m[5], ":"))
	switch {
	case verb == "inlining call to":
		inl.sites[place(path, line, m[3])] = true
	case filepath.IsAbs(path):
		// of the standard library or the module cache: not ours
	case verb == "cannot inline":
		inl.own[base], inl.judged[place(path, line, "")] = true, true
		inl.addWhy(base, strings.TrimPrefix(strings.TrimSpace(rest), ": "))
	default:
		inl.own[base], inl.judged[place(path, line, "")] = true, true
	}
}

// place is the absolute file, line and column of a call, or of a
// declaration without its column, to match the compiler's with ours.
func place(path, line, col string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return abs + ":" + line + ":" + col
}

// slowCalls returns the calls in h of the module's functions that the
// compiler did not inline: at the place of each call's parenthesis.
func slowCalls(h *hotFunc, inl inlining) []slowCall {
	var slow []slowCall
	ast.Inspect(h.fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := calleeName(call)
		if callee == "" || !inl.own[callee] {
			return true
		}
		at := h.fset.Position(call.Lparen)
		if !inl.sites[place(at.Filename, fmt.Sprint(at.Line), fmt.Sprint(at.Column))] {
			slow = append(slow, slowCall{at.Line, callee, inl.why[callee]})
		}
		return true
	})
	return slow
}

// builtins are called as functions, and never inlined as one.
var builtins = map[string]bool{"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true, "real": true, "recover": true}

func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if builtins[fun.Name] {
			return ""
		}
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.IndexExpr: // a generic function, instantiated
		return calleeName(&ast.CallExpr{Fun: fun.X})
	}
	return ""
}

// report tells h: its benchmarks, those not there, its calls not inlined.
func (h *hotFunc) report(w io.Writer) {
	fmt.Fprintf(w, "%s: %s: hot, measured by %s\n", h.pos, h.key, strings.Join(h.benchmarks, ", "))
	for _, b := range h.missing {
		fmt.Fprintf(w, "  benchmark %s not found in %s\n", b, h.dir)
	}
	if !h.compiled {
		fmt.Fprintln(w, "  not compiled here, being generic and not instantiated in the package or its tests: no inlining to tell")
	}
	for _, c := range h.slow {
		why := c.why
		if why == "" {
			why = "inlinable, but not inlined here"
		}
		fmt.Fprintf(w, "  line %d: %s not inlined: %s\n", c.line, c.callee, why)
	}
}
