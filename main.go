// Command abc keeps the functions of Go code small, by their ABC size:
// sqrt(A²+B²+C²) of their assignments, branches (calls) and conditions, as
// RuboCop's Metrics/AbcSize counts them, adapted to Go. It keeps them
// short, shallow and simple too: by their lines, but blank ones and
// comments; their deepest nesting of ifs, fors, switches and selects, an
// else if as deep as its if; and their cyclomatic complexity, as gocyclo
// counts it.
//
//	abc [-max 17] [-max-lines 40] [-max-nesting 3] [-max-cyclo 10]
//	    [-max-case 10] [-floor 0.3] [-baseline .abc-baseline] [-write]
//	    [-v] [-func name] [-json] [patterns]
//
// The patterns are directories, ./... by default; dir/... takes those
// under dir too. Tests are left out.
//
// It fails if a function is over a limit and not in the baseline, or
// larger than its baseline: the functions over it now may stay, or
// shrink. -write rewrites the baseline as the code is now.
//
// Regular code counts less: in the statements of a block, and the cases
// of a switch or a select, one alike in shape to one before it counts as
// much as it is new, by its ABC size, paths and lines. -floor is the
// likeness of any code, which discounts nothing.
//
// A case of a switch on a value or a type, a dispatch, is measured alone
// too, by -max-case, not less for its repeats: one that does much is to
// be moved out.
//
// -func name explains the functions of that name, or directory and name,
// instead: their sizes, and each statement and case by what it adds, less
// for a repeat, to show where to split them. -json writes that, or the
// problems, as JSON, for a model to act on.
//
// A comment on a function leaves metrics of it out, with the reason: a
// hot path measured slower once split. Its cases are left out only by
// naming the case metric. What is left out is of no limit, but kept in
// the baseline all the same: it may stay as large, or shrink, not grow.
//
//	//abc:disable abc,lines the hot path of every write, under mu
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	ms := metricFlags()
	floor := flag.Float64("floor", 0.3, "the likeness of siblings, 0 to 1, up to which none is a repeat")
	path := flag.String("baseline", ".abc-baseline", "the baseline file")
	rewrite := flag.Bool("write", false, "rewrite the baseline as the code is now")
	verbose := flag.Bool("v", false, "list every function over a limit")
	focus := flag.String("func", "", "explain the functions of this name, or directory and name: where their size is")
	asJSON := flag.Bool("json", false, "write what -func explains, or the problems, as JSON")
	flag.Parse()
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	opts := options{floor: *floor, focus: *focus}
	out := output{rewrite: *rewrite, verbose: *verbose, json: *asJSON}
	if err := run(patterns, *path, ms(), opts, out); err != nil {
		fmt.Fprintln(os.Stderr, "abc:", err)
		os.Exit(1)
	}
}

// metricFlags defines the limit of each metric, for the metrics after
// flag.Parse.
func metricFlags() func() []metric {
	abc := flag.Float64("max", 17, "the largest ABC size of a function not in the baseline")
	lines := flag.Int("max-lines", 40, "the most lines of a function, but blank ones and comments")
	nesting := flag.Int("max-nesting", 3, "the deepest nesting of ifs, fors, switches and selects")
	cyclo := flag.Int("max-cyclo", 10, "the largest cyclomatic complexity of a function")
	longCase := flag.Float64("max-case", 10, "the largest ABC size of a case of a dispatch")
	return func() []metric { return metrics(*abc, *lines, *nesting, *cyclo, *longCase) }
}

// An output is what run does with the sizes: rewrite the baseline, list
// those over a limit, and write JSON for a model, not text.
type output struct {
	rewrite, verbose, json bool
}

func run(patterns []string, path string, ms []metric, opts options, out output) error {
	scores, err := scan(patterns, names(ms), opts)
	if err != nil {
		return err
	}
	switch {
	case opts.focus != "" && out.json:
		return explainJSON(os.Stdout, scores, ms, opts)
	case opts.focus != "":
		return explainAll(os.Stdout, scores, ms, opts)
	}
	zs := sizes(scores, ms)
	if out.rewrite {
		return write(path, zs)
	}
	b, err := readBaseline(path)
	if err != nil {
		return err
	}
	if out.json {
		return reportJSON(os.Stdout, check(zs, b), shrunk(zs, b))
	}
	if out.verbose {
		list(zs)
	}
	if ks := shrunk(zs, b); len(ks) > 0 {
		fmt.Printf("%d sizes smaller than their baseline: abc -write to lower it\n", len(ks))
	}
	return report(check(zs, b))
}

// report prints the problems, a line a function, for a person or a model
// to act on, and fails if any. The functions furthest over a limit come
// first, as do their metrics.
func report(ps []problem) error {
	groups := byFunction(ps)
	for _, fps := range groups {
		fmt.Printf("%s: %s: %s\n", fps[0].z.s.pos, fps[0].z.s.key, describe(fps))
	}
	if len(ps) > 0 {
		return fmt.Errorf("%d functions too large, by %d sizes: %s", len(groups), len(ps), advice(ps))
	}
	return nil
}

// leftOutAdvice is for a size left out: a hot path or a critical section,
// kept whole on purpose, is not to be split to pass, but its baseline
// raised, if its reason still holds.
const leftOutAdvice = "keep those left out whole, and abc -write if their reason still holds"

// advice tells what to do of ps.
func advice(ps []problem) string {
	const split = "split them, return early, name their conditions, or move their cases out"
	left := 0
	for _, p := range ps {
		if p.z.leftOut {
			left++
		}
	}
	switch left {
	case 0:
		return split
	case len(ps):
		return leftOutAdvice
	}
	return split + "; " + leftOutAdvice
}

// byFunction groups ps, sorted, by function, in the order of their first.
func byFunction(ps []problem) [][]problem {
	var groups [][]problem
	at := map[string]int{}
	for _, p := range ps {
		i, seen := at[p.z.s.pos]
		if !seen {
			i = len(groups)
			at[p.z.s.pos] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], p)
	}
	return groups
}

// describe tells the metrics of a function over their bounds.
func describe(ps []problem) string {
	var parts []string
	for _, p := range ps {
		var part string
		switch p.what {
		case unrecorded:
			part = p.z.measured() + " (left out, " + unrecorded + ": abc -write)"
		case pastBaseline:
			part = fmt.Sprintf("%s > %g (%s)", p.z.measured(), p.bound, p.what)
			if p.z.leftOut {
				part = fmt.Sprintf("%s > %g (%s, left out: abc -write if it must grow)", p.z.measured(), p.bound, p.what)
			}
		default:
			part = fmt.Sprintf("%s > %g", p.z.measured(), p.bound)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// list prints the sizes over their limit, by metric, largest first.
func list(zs []size) {
	sorted := append([]size(nil), zs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].m.name != sorted[j].m.name {
			return sorted[i].m.name < sorted[j].m.name
		}
		return sorted[i].value > sorted[j].value
	})
	for _, z := range sorted {
		switch {
		case z.over() && z.leftOut:
			fmt.Println(z, "(left out)")
		case z.over():
			fmt.Println(z)
		}
	}
}
