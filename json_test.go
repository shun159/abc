package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainJSON(t *testing.T) {
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
	if err := explainJSON(&b, scores, metrics(17, 40, 3, 10, 10), opts); err != nil {
		t.Fatal(err)
	}
	var fs []jsonFunc
	if err := json.Unmarshal([]byte(b.String()), &fs); err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || len(fs[0].Parts) != 2 || fs[0].Sizes[0].Metric != "abc" {
		t.Fatalf("%+v", fs)
	}
	if p := fs[0].Parts[1]; p.Line != 5 || p.Repeat != 1 || p.Like != 4 || p.Size != 0 {
		t.Errorf("second part %+v; want line 5, all a repeat of line 4", p)
	}
}

func TestReportJSON(t *testing.T) {
	s := score{key: "x f", pos: "x.go:3:1", cases: []dispatchCase{{label: "1", line: 5, b: 11}}}
	ms := metrics(17, 40, 3, 10, 10)
	var b strings.Builder
	if err := reportJSON(&b, check(sizes([]score{s}, ms), baseline{}), nil); err == nil {
		t.Error("no failure for a case over its limit")
	}
	var r jsonReport
	if err := json.Unmarshal([]byte(b.String()), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Problems) != 1 || r.Problems[0].Metric != "case" || r.Problems[0].Case != "1" || r.Problems[0].Line != 5 {
		t.Errorf("%+v; want the case 1 at line 5", r.Problems)
	}
}
