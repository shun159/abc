package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// -diff rev measures a change: the functions of the Go files changed since
// rev, as they were and as they are, and what the change did to them all,
// to tell a split that simplifies from one that only spreads the code.

// A version is a function as it was, or as it is.
type version struct {
	s    score
	text string // its source, to tell whether it changed
	decl *ast.FuncDecl
	path string // of its file
}

// A changed is a function the change touched: before, after, or both.
type changed struct {
	key           string
	before, after *version
	split         *splitSigns // of a new function
}

// diffAgainst measures the functions of the files under the patterns
// changed since rev.
func diffAgainst(rev string, patterns []string, opts options) ([]changed, error) {
	files, err := changedFiles(rev)
	if err != nil {
		return nil, err
	}
	befores, afters := map[string]*version{}, map[string]*version{}
	for _, f := range files {
		if !underAny(f.before, patterns) && !underAny(f.after, patterns) {
			continue
		}
		if err := f.measure(rev, opts, befores, afters); err != nil {
			return nil, err
		}
	}
	return touched(befores, afters)
}

// A fileChange is a Go file changed: its path before, and after, either
// empty if it was added or deleted.
type fileChange struct{ before, after string }

// changedFiles lists the Go files changed since rev, but tests.
func changedFiles(rev string) ([]fileChange, error) {
	out, err := exec.Command("git", "diff", "--name-status", "-M", "--relative", rev, "--", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", rev, err)
	}
	var fs []fileChange
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f, ok := parseStatus(line); ok {
			fs = append(fs, f)
		}
	}
	added, err := untracked()
	return append(fs, added...), err
}

// untracked lists the Go files not yet added to git, but tests and those
// it ignores: added by the change, as far as it is written.
func untracked() ([]fileChange, error) {
	out, err := exec.Command("git", "ls-files", "--others", "--exclude-standard", "--", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var fs []fileChange
	for _, path := range strings.Fields(string(out)) {
		if isSource(filepath.Base(path)) {
			fs = append(fs, fileChange{after: path})
		}
	}
	return fs, nil
}

// parseStatus parses a line of git diff --name-status, of a source file.
func parseStatus(line string) (fileChange, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		return fileChange{}, false
	}
	var f fileChange
	switch status := fields[0]; {
	case strings.HasPrefix(status, "R") && len(fields) == 3:
		f = fileChange{fields[1], fields[2]}
	case status == "A":
		f.after = fields[1]
	case status == "D":
		f.before = fields[1]
	default:
		f = fileChange{fields[1], fields[1]}
	}
	return f, isSource(filepath.Base(f.before + f.after))
}

// measure measures the functions of f, as of rev and as now, into
// befores and afters.
func (f fileChange) measure(rev string, opts options, befores, afters map[string]*version) error {
	if f.before != "" {
		old, err := exec.Command("git", "show", rev+":./"+f.before).Output()
		if err != nil {
			return fmt.Errorf("git show %s:%s: %w", rev, f.before, err)
		}
		if err := versionsOf(f.before, old, opts, befores); err != nil {
			return err
		}
	}
	if f.after == "" {
		return nil
	}
	now, err := os.ReadFile(f.after)
	if err != nil {
		return err
	}
	return versionsOf(f.after, now, opts, afters)
}

// versionsOf measures the functions of the source data of the file path
// into vs.
func versionsOf(path string, data []byte, opts options, vs map[string]*version) error {
	fset, file, src, err := parseSource(path, data)
	if err != nil {
		return err
	}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		s := measureAt(fset, src, path, fn, opts)
		text := string(data[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])
		vs[s.key] = &version{s, text, fn, path}
	}
	return nil
}

// touched returns the functions changed, added or gone, by key, with the
// signs of a split of those added.
func touched(befores, afters map[string]*version) ([]changed, error) {
	keys := map[string]bool{}
	for k := range befores {
		keys[k] = true
	}
	for k := range afters {
		keys[k] = true
	}
	var cs []changed
	calls := callCounter{}
	for k := range keys {
		b, a := befores[k], afters[k]
		if b != nil && a != nil && b.text == a.text {
			continue
		}
		c := changed{key: k, before: b, after: a}
		if b == nil {
			signs, err := calls.signsOf(a)
			if err != nil {
				return nil, err
			}
			c.split = &signs
		}
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].key < cs[j].key })
	return cs, nil
}

// underAny tells whether the file path is under one of the patterns: in
// its directory, or below it for dir/....
func underAny(path string, patterns []string) bool {
	if path == "" {
		return false
	}
	dir := filepath.Dir(path)
	for _, p := range patterns {
		root, recursive := strings.CutSuffix(p, "/...")
		root = filepath.Clean(root)
		switch {
		case dir == root:
			return true
		case recursive && (root == "." || strings.HasPrefix(dir, root+string(filepath.Separator))):
			return true
		}
	}
	return false
}

// A changeSum is what a change did to the functions it touched: the
// largest ABC size, all of them, and how many, before and after.
type changeSum struct {
	largest, total, count [2]float64
	used1, tiny, through  int // new functions called once, of a statement, passing through
}

func summarize(cs []changed) changeSum {
	var sum changeSum
	for _, c := range cs {
		for i, v := range []*version{c.before, c.after} {
			if v == nil {
				continue
			}
			sum.largest[i] = max(sum.largest[i], v.s.abc())
			sum.total[i] += v.s.abc()
			sum.count[i]++
		}
		if c.split != nil {
			sum.used1 += btoi(c.split.calls == 1)
			sum.tiny += btoi(c.split.statements <= 1)
			sum.through += btoi(c.split.passThrough)
		}
	}
	return sum
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// reportDiff writes what the change did, a line a function, and the sum.
func reportDiff(w *bytes.Buffer, rev string, cs []changed) {
	sum := summarize(cs)
	fmt.Fprintf(w, "against %s: %d functions touched\n\n", rev, len(cs))
	fmt.Fprintf(w, "  %-15s %-15s %-13s %s\n", "abc", "lines", "cyclo", "function")
	for _, c := range cs {
		fmt.Fprintf(w, "  %-15s %-15s %-13s %s%s\n",
			change(c, func(s score) float64 { return s.abc() }),
			change(c, func(s score) float64 { return s.lines }),
			change(c, func(s score) float64 { return s.cyclo }),
			c.key, c.split.describe())
	}
	fmt.Fprintf(w, "\n  largest abc %g → %g, total abc %g → %g, functions %g → %g\n",
		sum.largest[0], sum.largest[1], round(sum.total[0]), round(sum.total[1]), sum.count[0], sum.count[1])
	if sum.used1+sum.tiny+sum.through > 0 {
		fmt.Fprintf(w, "  new functions: %d called at one place, %d of one statement, %d passing through\n",
			sum.used1, sum.tiny, sum.through)
	}
}

// change is a measure of c before → after, - where it is not.
func change(c changed, of func(score) float64) string {
	side := func(v *version) string {
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%g", round(of(v.s)))
	}
	return side(c.before) + " → " + side(c.after)
}

// runDiff measures the change since rev, and writes it, as text or JSON.
func runDiff(rev string, patterns []string, opts options, asJSON bool) error {
	cs, err := diffAgainst(rev, patterns, opts)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(os.Stdout, diffJSON(rev, cs))
	}
	var b bytes.Buffer
	reportDiff(&b, rev, cs)
	_, err = os.Stdout.Write(b.Bytes())
	return err
}
