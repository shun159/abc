package main

import (
	"os"
	"strings"
	"testing"
)

func TestHot(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"go.mod": "module x\n\ngo 1.26\n",
		"x.go": `package x

// F is hot.
//
//abc:hot BenchmarkF,BenchmarkGone the path of every call
func F(n int) int { return small(n) + big(n) }

func small(n int) int { return n + 1 }

func big(n int) (s int) {
	defer func() { s++ }() // never inlined
	return n * 2
}
`,
		"x_test.go": "package x\n\nimport \"testing\"\n\nfunc BenchmarkF(b *testing.B) {\n\tfor b.Loop() {\n\t\tF(10)\n\t}\n}\n",
	}
	for name, src := range files {
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var b strings.Builder
	err := runHot([]string{"."}, &b)
	if err == nil {
		t.Error("no failure for a benchmark not there")
	}
	out := b.String()
	for _, want := range []string{"hot, measured by BenchmarkF, BenchmarkGone", "benchmark BenchmarkGone not found", "big not inlined"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "small not inlined") || strings.Contains(out, "make not inlined") {
		t.Errorf("an inlined call, or a builtin, told:\n%s", out)
	}
}
