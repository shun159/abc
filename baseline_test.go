package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestALeftOutSizeIsKeptNotToGrow(t *testing.T) {
	ms := metrics(17, 40, 3, 10, 10)
	at := func(a float64) []size {
		s := score{key: "x f", pos: "x.go:3:1", a: a, disabled: []string{"abc"}}
		return sizes([]score{s}, ms)
	}
	if ps := check(at(5), baseline{}); len(ps) != 1 || ps[0].what != unrecorded {
		t.Fatalf("%v; want it not in the baseline, though under its limit", ps)
	}
	path := filepath.Join(t.TempDir(), ".abc-baseline")
	if err := write(path, at(30)); err != nil {
		t.Fatal(err)
	}
	b, err := readBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if b["x f"] != 30 {
		t.Fatalf("baseline %v; want x f at 30", b)
	}
	if ps := check(at(30), b); len(ps) != 0 {
		t.Errorf("%v; want none as large as its baseline", ps)
	}
	if ps := check(at(31), b); len(ps) != 1 || ps[0].what != pastBaseline {
		t.Errorf("%v; want it past its baseline", ps)
	}
}

func TestALeftOutSizeIsNotToSplit(t *testing.T) {
	left := problem{z: size{leftOut: true}}
	other := problem{}
	if got := advice([]problem{left}); got != leftOutAdvice {
		t.Errorf("advice %q; want only %q", got, leftOutAdvice)
	}
	if got := advice([]problem{other, left}); !strings.HasSuffix(got, leftOutAdvice) || !strings.HasPrefix(got, "split") {
		t.Errorf("advice %q; want to split the others, and keep those left out", got)
	}
}
