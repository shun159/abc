package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDiffTellsASplit(t *testing.T) {
	t.Chdir(t.TempDir())
	write := func(src string) {
		if err := os.WriteFile(filepath.Join(".", "x.go"), []byte("package x\n\n"+src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, "init", "-q")
	write(`func f(a, b int) int {
	if a > b {
		a, b = b, a
	}
	return g(a) + g(b)
}

func g(x int) int { return x * 2 }
`)
	git(t, "add", ".")
	git(t, "commit", "-q", "-m", "one")
	write(`func f(a, b int) int {
	a, b = ordered(a, b)
	return twice(a, b)
}

func ordered(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

func twice(a, b int) int { return sum(a, b) }

func sum(a, b int) int { return g(a) + g(b) }

func g(x int) int { return x * 2 }
`)
	if err := os.WriteFile("y.go", []byte("package x\n\nfunc h() int { return g(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, err := diffAgainst("HEAD", []string{"./..."}, options{floor: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]changed{}
	for _, c := range cs {
		keys[c.key] = c
	}
	if len(cs) != 5 || keys[". f"].before == nil || keys[". g"].key != "" || keys[". h"].split == nil {
		t.Fatalf("touched %v; want f, and ordered, twice, sum new, h of a file not added yet, not g", cs)
	}
	twice := keys[". twice"].split
	if twice == nil || twice.calls != 1 || twice.statements != 1 || !twice.passThrough {
		t.Errorf("twice: %+v; want called once, of one statement, passing through", twice)
	}
	if sum := summarize(cs); sum.count != [2]float64{1, 5} || sum.used1 != 3 {
		t.Errorf("summary %+v; want 1 → 5 functions, 3 called once", sum)
	}
	var md bytes.Buffer
	markdownDiff(&md, cs)
	for _, want := range []string{"| function | abc |", "| `. twice` | - → ", "passing through |", "functions 1 → 5."} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("no %q in\n%s", want, md.String())
		}
	}
}
