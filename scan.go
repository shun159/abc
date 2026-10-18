package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// scan measures the functions of the Go files under the patterns: a
// directory, or one followed by /... for those under it too. Tests are
// left out. known is the names of the metrics.
func scan(patterns []string, known []string, opts options) ([]score, error) {
	var all []score
	for _, p := range patterns {
		dir, recursive := strings.CutSuffix(p, "/...")
		files, err := goFiles(dir, recursive)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			scores, err := scanFile(f, known, opts)
			if err != nil {
				return nil, err
			}
			all = append(all, scores...)
		}
	}
	return all, nil
}

// goFiles lists the Go files of dir, but tests, and those of the
// directories under it if recursive, but hidden ones and testdata.
func goFiles(dir string, recursive bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return enter(dir, path, d.Name(), recursive)
		}
		if isSource(d.Name()) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// enter tells WalkDir whether to go into the directory path.
func enter(root, path, name string, recursive bool) error {
	if path == root {
		return nil
	}
	skipped := strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata"
	if !recursive || skipped {
		return filepath.SkipDir
	}
	return nil
}

func isSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// options are how functions are measured: floor is the likeness up to
// which siblings are not repeats, and focus names the functions to
// explain, if any.
type options struct {
	floor float64
	focus string
}

// focuses tells whether the function of key, its directory and name, is
// one to explain: by its name, or its directory and name.
func (o options) focuses(key string) bool {
	if o.focus == "" {
		return false
	}
	return key == o.focus || strings.HasSuffix(key, " "+o.focus)
}

// scanFile measures the functions of the file path.
func scanFile(path string, known []string, opts options) ([]score, error) {
	fset, f, src, err := parse(path)
	if err != nil {
		return nil, err
	}
	var scores []score
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		metrics, err := disabled(fn, known)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fset.Position(fn.Pos()), err)
		}
		s := measureAt(fset, src, path, fn, opts)
		s.disabled = metrics
		scores = append(scores, s)
	}
	return scores, nil
}

// measureAt measures fn, of the file path of lines src, and names it and
// places its cases, and its parts if it is to be explained.
func measureAt(fset *token.FileSet, src []string, path string, fn *ast.FuncDecl, opts options) score {
	key := filepath.ToSlash(filepath.Dir(path)) + " " + name(fn)
	s := measure(fn, opts.floor, spanOf(fset, src), opts.focuses(key))
	line := func(n ast.Node) int { return fset.Position(n.Pos()).Line }
	for i := range s.cases {
		s.cases[i].line = line(s.cases[i].at)
	}
	for i := range s.parts {
		p := &s.parts[i]
		at := fset.Position(p.at.Pos())
		p.line, p.code = at.Line, strings.TrimSpace(src[at.Line-1][at.Column-1:])
		if p.like != nil {
			p.likeLine = line(p.like)
		}
	}
	s.key, s.pos = key, fset.Position(fn.Pos()).String()
	return s
}

// parse parses the file path, and splits it in lines.
func parse(path string) (*token.FileSet, *ast.File, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	return parseSource(path, data)
}

// parseSource parses data, the source of the file path, and splits it in
// lines.
func parseSource(path string, data []byte) (*token.FileSet, *ast.File, []string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	return fset, f, strings.Split(string(data), "\n"), err
}
